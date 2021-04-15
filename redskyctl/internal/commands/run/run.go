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
	"os/exec"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/internal/experiment"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commander"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/kustomize"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"github.com/thestormforge/optimize-go/pkg/config"
)

type Options struct {
	// Config is the Red Sky Configuration
	Config *config.RedSkyConfig
	// ExperimentsAPI is used to interact with the Red Sky Experiments API
	ExperimentsAPI experimentsv1alpha1.API

	Verbose bool

	Generator experiment.Generator
	maybeQuit bool
	lastErr   error
	status    string

	initializationModel initializationModel
	generationModel     generationModel
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

	return cmd
}

func (o *Options) Init() tea.Cmd {
	// None of this works without the real kubectl
	o.Generator.FilterOptions.KubectlExecutor = func(cmd *exec.Cmd) ([]byte, error) { return cmd.Output() }

	// Make sure there is a value for the controller image
	if o.initializationModel.ControllerImage == "" {
		o.initializationModel.ControllerImage = kustomize.BuildImage
	}

	// Make sure all the models are in a valid state
	o.initializeModel()

	// Run a bunch of commands to get things started
	return tea.Batch(
		o.checkBuildVersion,
		o.checkKubectlVersion,
		o.checkForgeVersion,
		o.checkControllerVersion,
		o.checkAuthorization,
		o.listKubernetesNamespaces,
	)
}

func (o *Options) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// User initiated exit: ctrl+c or be prompted when you hit esc
		switch msg.Type {

		case tea.KeyEsc:
			if o.status != "" {
				o.status = ""
			} else {
				o.maybeQuit = true
			}
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

	case statusMsg:
		o.status = string(msg)

	case versionMsg:
		// Run init if the controller version comes back unknown
		if msg.isControllerUnavailable() {
			cmds = append(cmds, o.initializeController)
		}

		// If the forge CLI is present, get the list of test case names
		if msg.isForgeAvailable() {
			cmds = append(cmds, o.listStormForgerTestCaseNames)
		}

	case applicationMsg:
		// An application indicates it is time to generate an experiment
		o.Generator.Application = redskyappsv1alpha1.Application(msg)
		cmds = append(cmds, o.generateExperiment)

	case experimentStatusMsg:
		switch msg {
		case expConfirmed:
			// Create the experiment (i.e. run it)
			cmds = append(cmds, o.createExperiment)
		case expCreated:
			// Initiate the status refresh
			cmds = append(cmds, o.refreshTick())
		}

	case trialsMsg:
		// If we got a status refresh, initial another
		cmds = append(cmds, o.refreshTick())

	case refreshStatusMsg:
		// Handle a refreshTick by fetching the trials
		cmds = append(cmds, o.refreshExperimentStatus)

	case error:
		// Handle errors so any returning tea.Msg can just return an error
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

	o.generationModel, cmd = o.generationModel.Update(msg)
	cmds = append(cmds, cmd)

	o.previewModel, cmd = o.previewModel.Update(msg)
	cmds = append(cmds, cmd)

	o.runModel, cmd = o.runModel.Update(msg)
	cmds = append(cmds, cmd)

	return o, tea.Batch(cmds...)
}
