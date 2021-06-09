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
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	konjurev1beta2 "github.com/thestormforge/konjure/pkg/api/core/v1beta2"
	"github.com/thestormforge/konjure/pkg/konjure"
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/run/form"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/run/internal"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/kustomize"
	"github.com/thestormforge/optimize-controller/v2/internal/experiment"
	"github.com/thestormforge/optimize-controller/v2/internal/version"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"github.com/thestormforge/optimize-go/pkg/config"
	"github.com/yujunz/go-getter"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
)

type Options struct {
	// Config is the Optimize Configuration.
	Config *config.OptimizeConfig
	// ExperimentsAPI is used to interact with the Optimize Experiments API.
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
		} else {
			cmds = append(cmds, func() tea.Msg { return internal.PerformanceTestAuthorizationMsg(internal.AuthorizationInvalid) })
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

	case internal.InitializationFinished:
		// If the generation form is enabled, start it, otherwise skip ahead
		if o.generatorModel.form().Enabled() {
			cmds = append(cmds, form.Start)
		} else {
			cmds = append(cmds, o.generateExperiment)
		}

	case form.FinishedMsg:
		if o.generatorModel.form().Focused() {
			// We hit the end of the generator form, trigger generation
			cmds = append(cmds, o.generateExperiment)
		}

	case internal.ExperimentReadyMsg:
		switch {
		case msg.Cluster:
			// User wants the experiment run in the cluster
			cmds = append(cmds, o.createExperimentInCluster)
		case msg.File:
			// User wants the experiment written to disk
			cmds = append(cmds, o.createExperimentInFile)
		}

	case internal.ExperimentCreatedMsg:
		if msg.Filename == "" {
			// The experiment is in the cluster, start refreshing the trial status
			cmds = append(cmds, o.refreshTrialsTick())
		}

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
	err := cobra.MaximumNArgs(1)(nil, args)
	if err != nil {
		return err
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	var path string
	switch len(args) {
	case 0:
		path = filepath.Join(wd, "app.yaml")
	case 1:
		path, err = readIntoApplication(args[0], wd, &o.Generator.Application)
		if err != nil {
			return err
		}
	}

	meta.SetMetaDataAnnotation(&o.Generator.Application.ObjectMeta, kioutil.PathAnnotation, path)
	return nil
}

// applyToApp takes all of the what is on the model and applies it to an application.
func (m generatorModel) applyToApp(app *optimizeappsv1alpha1.Application) {
	if m.NamespaceInput.Enabled() {

		// TODO We need a better way to set the name/namespace of the application
		if namespaces := m.NamespaceInput.Values(); len(namespaces) == 1 {
			app.Name = namespaces[0]
			app.Namespace = namespaces[0]
		}

		for i, ns := range m.NamespaceInput.Values() {
			app.Resources = append(app.Resources, konjure.Resource{
				Kubernetes: &konjurev1beta2.Kubernetes{
					Namespaces: []string{ns},
					Selector:   m.LabelSelectorInputs[i].Value(),
				},
			})
		}
	}

	if m.StormForgerTestCaseInput.Enabled() {
		if testCase := m.StormForgerTestCaseInput.Value(); testCase != "" {
			app.Scenarios = append(app.Scenarios, optimizeappsv1alpha1.Scenario{
				StormForger: &optimizeappsv1alpha1.StormForgerScenario{
					TestCase: testCase,
				},
			})
		}
	}

	if m.LocustfileInput.Enabled() {
		if locustfile := m.LocustfileInput.Value(); locustfile != "" {
			app.Scenarios = append(app.Scenarios, optimizeappsv1alpha1.Scenario{
				Locust: &optimizeappsv1alpha1.LocustScenario{
					Locustfile: locustfile,
				},
			})
		}
	}

	if m.IngressURLInput.Enabled() {
		if u := m.IngressURLInput.Value(); u != "" {
			app.Ingress = &optimizeappsv1alpha1.Ingress{
				URL: u,
			}
		}
	}

	if m.ContainerResourcesSelectorInput.Enabled() {
		// NOTE: We do not check for an empty value here to match the expectation
		// that empty matches everything (even if other parameters have been specified).
		// We CANNOT rely on the default behavior of the generator for this, since
		// the presence of any parameter bypasses the default inclusion of container resources.
		app.Parameters = append(app.Parameters, optimizeappsv1alpha1.Parameter{
			ContainerResources: &optimizeappsv1alpha1.ContainerResources{
				Selector: m.ContainerResourcesSelectorInput.Value(),
			},
		})
	}

	if m.ReplicasSelectorInput.Enabled() {
		if sel := m.ReplicasSelectorInput.Value(); sel != "" {
			app.Parameters = append(app.Parameters, optimizeappsv1alpha1.Parameter{
				Replicas: &optimizeappsv1alpha1.Replicas{
					Selector: sel,
				},
			})
		}
	}

	if m.ObjectiveInput.Enabled() {
		app.Objectives = append(app.Objectives, optimizeappsv1alpha1.Objective{})
		for _, goal := range m.ObjectiveInput.Values() {
			app.Objectives[0].Goals = append(app.Objectives[0].Goals, optimizeappsv1alpha1.Goal{Name: goal})
		}
	}

}

// readIntoApplication reads the supplied source relative to a working directory.
func readIntoApplication(src, wd string, app *optimizeappsv1alpha1.Application) (string, error) {
	path := filepath.Join(wd, src)

	// Try to use go-getter to support non-file inputs
	u, err := getter.Detect(src, wd, getter.Detectors)
	if err == nil && !strings.HasPrefix(u, "file:") {
		path = filepath.Join(os.TempDir(), fmt.Sprintf("go-getter-application-md5-%x", md5.Sum([]byte(src))))
		defer func() { _ = os.Remove(path) }()

		c := getter.Client{
			Src: src,
			Dst: path,
			Pwd: wd,
		}
		if err := c.Get(); err != nil {
			return "", fmt.Errorf("unable to read application: %w", err)
		}
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}

	if err := commander.NewResourceReader().ReadInto(f, app); err != nil {
		return "", err
	}

	return path, nil
}
