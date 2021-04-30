/*
Copyright 2021 GramLabs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package run

import (
	"crypto/md5"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	konjurev1beta2 "github.com/thestormforge/konjure/pkg/api/core/v1beta2"
	"github.com/thestormforge/konjure/pkg/konjure"
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/internal/experiment"
	"github.com/thestormforge/optimize-controller/internal/version"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commander"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/run/form"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/run/internal"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/kustomize"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"github.com/thestormforge/optimize-go/pkg/config"
	"github.com/yujunz/go-getter"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
)

type Options struct {
	// Config is the Red Sky Configuration.
	Config *config.RedSkyConfig
	// ExperimentsAPI is used to interact with the Red Sky Experiments API.
	ExperimentsAPI experimentsv1alpha1.API

	// Flag indicating we should print verbose prompts.
	Verbose bool
	// Flag indicating we should print debug views.
	Debug bool
	// Generator used to create experiments.
	Generator experiment.Generator

	maybeQuit bool
	lastErr   error

	initializationModel initializationModel
	generatorModel      generatorModel
	previewModel        previewModel
	runModel            runModel
}

// NewCommand creates a new command for running experiments.
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run an experiment",

		PreRunE: func(cmd *cobra.Command, args []string) error {
			o.initializationModel.CommandName = cmd.Root().Name()
			o.Generator.FilterOptions.DefaultReader = cmd.InOrStdin()
			if err := o.ReadApplication(args); err != nil {
				return err
			}
			return commander.SetExperimentsAPI(&o.ExperimentsAPI, o.Config, cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return tea.NewProgram(o,
				tea.WithInput(cmd.InOrStdin()),
				tea.WithOutput(cmd.OutOrStderr()),
			).Start()
		},
	}

	cmd.Flags().BoolVarP(&o.Verbose, "verbose", "v", o.Verbose, "display verbose prompts")
	cmd.Flags().BoolVar(&o.Verbose, "debug", o.Debug, "display debug information")
	_ = cmd.Flags().MarkHidden("debug")

	// TODO Un-hide this once we have distinct verbose prompts
	_ = cmd.Flags().MarkHidden("verbose")

	return cmd
}

func (o *Options) Init() tea.Cmd {
	// None of this works without the real kubectl
	o.Generator.FilterOptions.KubectlExecutor = func(cmd *exec.Cmd) ([]byte, error) { return cmd.Output() }

	// Capture the build version number
	o.initializationModel.BuildVersion = version.GetInfo().String()

	// Make sure there is a value for the controller image
	if o.initializationModel.ControllerImage == "" {
		o.initializationModel.ControllerImage = kustomize.BuildImage
	}

	// Make sure all the models are in a valid state
	o.initializeModel()

	// Run a bunch of commands to get things started
	return tea.Batch(
		o.checkKubectlVersion,
		o.checkForgeVersion,
		o.checkControllerVersion,
		o.checkOptimizeAuthorization,
	)
}

func (o *Options) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {

	case tea.KeyMsg:
		// User initiated exit: ctrl+c or be prompted when you hit esc
		switch msg.Type {

		case tea.KeyEsc:
			o.maybeQuit = true
		case tea.KeyCtrlC:
			return o, tea.Quit

		default:
			if o.maybeQuit {
				switch msg.String() {
				case "y", "Y", "enter":
					return o, tea.Quit
				case "n", "N":
					o.maybeQuit = false
					return o, tea.Batch(textinput.Blink) // Restart the blinking cursor
				}
			}
		}

	case internal.KubectlVersionMsg:
		// If kubectl is available, get the namespaces
		if internal.NewVersion(msg).Available() {
			cmds = append(cmds, o.listKubernetesNamespaces)
		}

	case internal.ForgeVersionMsg:
		// If forge is available, check the authorization
		if internal.NewVersion(msg).Available() {
			cmds = append(cmds, o.checkPerformanceTestAuthorization)
		}

	case internal.OptimizeControllerVersionMsg:
		// Run init if the controller version comes back unknown
		if !internal.NewVersion(msg).Available() {
			cmds = append(cmds, o.initializeController)
		}

	case internal.PerformanceTestAuthorizationMsg:
		// If forge is authorized, get the test case names
		if internal.AuthorizationStatus(msg) == internal.AuthorizationValid {
			cmds = append(cmds, o.listStormForgerTestCaseNames)
		}

	case form.FinishedMsg:
		// We hit the end of the generator form, apply the changes and trigger generation
		o.applyGeneratorModel()
		cmds = append(cmds, o.generateExperiment)

	case internal.ExperimentConfirmedMsg:
		// The user confirmed they are ready to run the experiment
		cmds = append(cmds, o.createExperiment)

	case internal.ExperimentCreatedMsg:
		// The experiment is in the cluster, start refreshing the trial status
		cmds = append(cmds, o.refreshTrialsTick())

	case internal.TrialsMsg:
		// If we got a status refresh, initiate another
		cmds = append(cmds, o.refreshTrialsTick())

	case internal.TrialsRefreshMsg:
		// Refresh the trials list
		cmds = append(cmds, o.refreshTrials)

	case error:
		// Handle errors so any command returning tea.Msg can just return an error
		o.lastErr = msg
		return o, tea.Quit

	}

	// If the user hit esc and might quit, don't bother updating the rest of the model
	if o.maybeQuit {
		return o, nil
	}

	// Update the child models
	var cmd tea.Cmd

	o.initializationModel, cmd = o.initializationModel.Update(msg)
	cmds = append(cmds, cmd)

	// We need to see the latest init model state _before_ we let the generator update
	o.updateGeneratorForm()

	o.generatorModel, cmd = o.generatorModel.Update(msg)
	cmds = append(cmds, cmd)

	o.previewModel, cmd = o.previewModel.Update(msg)
	cmds = append(cmds, cmd)

	o.runModel, cmd = o.runModel.Update(msg)
	cmds = append(cmds, cmd)

	return o, tea.Batch(cmds...)
}

// ReadApplication uses go-getter from the current directory to fetch an application definition.
func (o *Options) ReadApplication(args []string) error {
	c := getter.Client{}

	err := cobra.MaximumNArgs(1)(nil, args)
	if err != nil {
		return err
	}

	c.Pwd, err = os.Getwd()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		meta.SetMetaDataAnnotation(&o.Generator.Application.ObjectMeta, kioutil.PathAnnotation, filepath.Join(c.Pwd, "app.yaml"))
		return nil
	}

	c.Src = args[0]
	c.Dst = filepath.Join(os.TempDir(), fmt.Sprintf("read-application-%x", md5.Sum([]byte(c.Src))))
	defer os.Remove(c.Dst)

	if err := c.Get(); err != nil {
		return fmt.Errorf("unable to read application: %w", err)
	}

	f, err := os.Open(c.Dst)
	if err != nil {
		return err
	}

	return commander.NewResourceReader().ReadInto(f, &o.Generator.Application)
}

// applyGeneratorModel takes all of the what is on the generatorModel and applies
// it to the application in the experiment generator.
func (o *Options) applyGeneratorModel() {
	if o.generatorModel.NamespaceInput.Enabled() {

		// TODO DEMO ONLY HACK
		if namespaces := o.generatorModel.NamespaceInput.Values(); len(namespaces) == 1 {
			o.Generator.Application.Name = namespaces[0]
			o.Generator.Application.Namespace = namespaces[0]
		}

		for i, ns := range o.generatorModel.NamespaceInput.Values() {
			o.Generator.Application.Resources = append(o.Generator.Application.Resources, konjure.Resource{
				Kubernetes: &konjurev1beta2.Kubernetes{
					Namespaces: []string{ns},
					Selector:   o.generatorModel.LabelSelectorInputs[i].Value(),
				},
			})
		}
	}

	if o.generatorModel.StormForgerTestCaseInput.Enabled() {
		if testCase := o.generatorModel.StormForgerTestCaseInput.Value(); testCase != "" {
			o.Generator.Application.Scenarios = append(o.Generator.Application.Scenarios, redskyappsv1alpha1.Scenario{
				StormForger: &redskyappsv1alpha1.StormForgerScenario{
					TestCase: testCase,
				},
			})
		}
	}

	if o.generatorModel.LocustfileInput.Enabled() {
		if locustfile := o.generatorModel.LocustfileInput.Value(); locustfile != "" {
			o.Generator.Application.Scenarios = append(o.Generator.Application.Scenarios, redskyappsv1alpha1.Scenario{
				Locust: &redskyappsv1alpha1.LocustScenario{
					Locustfile: locustfile,
				},
			})
		}
	}

	if o.generatorModel.IngressURLInput.Enabled() {
		if u := o.generatorModel.IngressURLInput.Value(); u != "" {
			o.Generator.Application.Ingress = &redskyappsv1alpha1.Ingress{
				URL: u,
			}
		}
	}

	if o.generatorModel.ContainerResourcesSelectorInput.Enabled() {
		if sel := o.generatorModel.ContainerResourcesSelectorInput.Value(); sel != "" {
			o.Generator.Application.Parameters = append(o.Generator.Application.Parameters, redskyappsv1alpha1.Parameter{
				ContainerResources: &redskyappsv1alpha1.ContainerResources{
					Selector: sel,
				},
			})
		}
	}

	if o.generatorModel.ReplicasSelectorInput.Enabled() {
		if sel := o.generatorModel.ReplicasSelectorInput.Value(); sel != "" {
			o.Generator.Application.Parameters = append(o.Generator.Application.Parameters, redskyappsv1alpha1.Parameter{
				Replicas: &redskyappsv1alpha1.Replicas{
					Selector: sel,
				},
			})
		}
	}

	if o.generatorModel.ObjectiveInput.Enabled() {
		o.Generator.Application.Objectives = append(o.Generator.Application.Objectives, redskyappsv1alpha1.Objective{})
		for _, goal := range o.generatorModel.ObjectiveInput.Values() {
			o.Generator.Application.Objectives[0].Goals = append(o.Generator.Application.Objectives[0].Goals, redskyappsv1alpha1.Goal{Name: goal})
		}
	}

}