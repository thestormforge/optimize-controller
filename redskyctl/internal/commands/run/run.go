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
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/internal/experiment"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commander"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/kustomize"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"github.com/thestormforge/optimize-go/pkg/config"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
)

type Options struct {
	// Config is the Red Sky Configuration
	Config *config.RedSkyConfig
	// ExperimentsAPI is used to interact with the Red Sky Experiments API
	ExperimentsAPI experimentsv1alpha1.API
	// Flag indicating that we should produce some extra output
	Debug bool

	Generator experiment.Generator
	maybeQuit bool
	lastErr   error

	welcomeModel    welcomeModel
	generationModel generationModel
	previewModel    previewModel
	runModel        runModel
}

// NewCommand creates a new command for running experiments.
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run an experiment",

		PreRunE: func(cmd *cobra.Command, args []string) error {
			o.welcomeModel.CommandName = cmd.Root().Name()
			o.Generator.FilterOptions.DefaultReader = cmd.InOrStdin()
			return commander.SetExperimentsAPI(&o.ExperimentsAPI, o.Config, cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return tea.NewProgram(o,
				tea.WithInput(cmd.InOrStdin()),
				tea.WithOutput(cmd.OutOrStderr()),
			).Start()
		},
	}

	cmd.Flags().BoolVar(&o.Debug, "debug", false, "enable debug mode")
	_ = cmd.Flags().MarkHidden("debug")

	return cmd
}

func (o *Options) Init() tea.Cmd {
	// None of this works without the real kubectl
	o.Generator.FilterOptions.KubectlExecutor = func(cmd *exec.Cmd) ([]byte, error) { return cmd.Output() }

	// Try to record the working directory on the application
	if wd, err := os.Getwd(); err == nil {
		path := filepath.Join(wd, "app.yaml")
		meta.SetMetaDataAnnotation(&o.Generator.Application.ObjectMeta, kioutil.PathAnnotation, path)
	}

	// Make sure there is a value for the controller image
	if o.welcomeModel.ControllerImage == "" {
		o.welcomeModel.ControllerImage = kustomize.BuildImage
	}

	// Make sure all the models are in a valid state
	o.initializeModel()

	return tea.Batch(
		textinput.Blink,
		spinner.Tick,
		o.checkBuildVersion,
		// TODO Should we break up the rest so we are getting something on screen quicker?
		o.checkForgeVersion,
		o.checkKubectlVersion,
		o.checkControllerVersion,
		o.checkAuthorization,
		o.listKubernetesNamespaces,
	)
}

func (o *Options) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Check for messages we need to handle at the top level
	switch msg := msg.(type) {

	case tea.KeyMsg:
		// User initiated exit: ctrl+c or be prompted when you hit esc
		switch msg.String() {
		case "ctrl+c":
			return o, tea.Quit
		case "esc":
			o.maybeQuit = true
			return o, nil
		case "y", "Y", "enter":
			if o.maybeQuit {
				return o, tea.Quit
			}
		case "n", "N":
			if o.maybeQuit {
				o.maybeQuit = false
			}
		}

	case versionMsg:
		// Run init if the controller version comes back unknown
		if msg.Controller.Version == "unknown" {
			cmds = append(cmds, o.initializeController)
		}

		// If the forge CLI is present, get the list of test case names
		if msg.Forge != "" && msg.Forge != "unknown" {
			cmds = append(cmds, o.listStormForgerTestCaseNames)
		}

	// These messages all modify the application definition used to generate the experiment
	case resourceMsg:
		o.Generator.Application.Resources = append(o.Generator.Application.Resources, msg...)
	case scenarioMsg:
		o.Generator.Application.Scenarios = append(o.Generator.Application.Scenarios, msg...)
	case parameterMsg:
		o.Generator.Application.Parameters = append(o.Generator.Application.Parameters, msg...)
	case objectiveMsg:
		o.Generator.Application.Objectives = append(o.Generator.Application.Objectives, msg...)
	case ingressMsg:
		o.Generator.Application.Ingress = new(redskyappsv1alpha1.Ingress)
		*o.Generator.Application.Ingress = redskyappsv1alpha1.Ingress(msg)

	// Experiment lifecycle
	case experimentStatusMsg:
		switch msg {
		case expPreCreate:
			cmds = append(cmds, o.generateExperiment)
		case expConfirmed:
			cmds = append(cmds, o.createExperiment)
		case expCreated:
			cmds = append(cmds, o.refreshTick())
		}

	// Handle the timed status checks after the experiment starts
	case refreshStatusMsg:
		cmds = append(cmds, o.refreshExperimentStatus)
	case trialsMsg:
		cmds = append(cmds, o.refreshTick())

	// Handle errors so any returning tea.Msg can just return an error
	case error:
		o.lastErr = msg
		return o, tea.Quit

	}

	// If the user hit esc and might quit, don't bother updating the rest of the model
	if o.maybeQuit {
		return o, nil
	}

	// Update the child models
	var cmd tea.Cmd

	o.welcomeModel, cmd = o.welcomeModel.Update(msg)
	cmds = append(cmds, cmd)

	o.generationModel, cmd = o.generationModel.Update(msg)
	cmds = append(cmds, cmd)

	o.previewModel, cmd = o.previewModel.Update(msg)
	cmds = append(cmds, cmd)

	o.runModel, cmd = o.runModel.Update(msg)
	cmds = append(cmds, cmd)

	return o, tea.Batch(cmds...)
}
