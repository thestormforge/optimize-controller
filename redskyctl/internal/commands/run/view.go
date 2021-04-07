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
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/run/choiceinput"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/run/multichoiceinput"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// initializeModel is invoked before the program is started to ensure things
// are in a valid state prior to starting.
func (o *Options) initializeModel() {

	o.generationModel.TestCaseInput = choiceinput.NewModel()
	o.generationModel.TestCaseInput.Prompt = "Please select a StormForger test case to optimize for: "
	o.generationModel.TestCaseInput.LoadingMessage = "Fetching test cases from StormForger ..."
	o.generationModel.TestCaseInput.Instructions = "up/down: select  |  enter: continue"

	o.generationModel.LocustNameInput = textinput.NewModel()
	o.generationModel.LocustNameInput.Prompt = "Please enter a name for your Locust test: "

	o.generationModel.LocustfileInput = textinput.NewModel()
	o.generationModel.LocustfileInput.Prompt = "Enter the location of the locustfile.py you would like to run: "

	o.generationModel.NamespaceInput = multichoiceinput.NewModel()
	o.generationModel.NamespaceInput.Prompt = "Please select the Kubernetes namespace where your application is running: "
	o.generationModel.NamespaceInput.LoadingMessage = "Fetching namespaces from Kubernetes ..."
	o.generationModel.NamespaceInput.Instructions = "up/down: select  |  space: choose  |  enter: continue"
	o.generationModel.NamespaceInput.Required = true

	o.generationModel.LabelSelectorTemplate = textinput.NewModel()
	o.generationModel.LabelSelectorTemplate.Prompt = "Specify the label selector for your application resources in the '%s' namespace:\n"
	o.generationModel.LabelSelectorTemplate.Placeholder = "leave blank to select all resources"

	o.generationModel.IngressURLInput = textinput.NewModel()
	o.generationModel.IngressURLInput.Prompt = "Enter the URL of the endpoint to test: "

	o.generationModel.ContainerResourcesSelectorInput = textinput.NewModel()
	o.generationModel.ContainerResourcesSelectorInput.Prompt = "Specify the label selector matching resources which should have their memory and CPU optimized:\n"
	o.generationModel.ContainerResourcesSelectorInput.Placeholder = "leave blank to select all resources"

	o.generationModel.ReplicasSelectorInput = textinput.NewModel()
	o.generationModel.ReplicasSelectorInput.Prompt = "Specify the label selector matching resources which can be scaled horizontally:\n"
	o.generationModel.ReplicasSelectorInput.Placeholder = "leave blank to select NO resources"

	o.generationModel.ObjectiveInput = multichoiceinput.NewModel()
	o.generationModel.ObjectiveInput.Prompt = "Please select objectives to optimize: "
	o.generationModel.ObjectiveInput.Instructions = "up/down: select  |  space: choose  |  enter: continue"
	o.generationModel.ObjectiveInput.Choices = []string{
		"cost",
		"p50-latency",
		"p95-latency",
		"p99-latency",
	}
	o.generationModel.ObjectiveInput.Select(0)
	o.generationModel.ObjectiveInput.Select(2)

}

// View returns a full rendering of the current state. This method is called
// from the event loop and must not block, it must return as fast as possible.
func (o *Options) View() string {
	var lines []string

	// The "runModel" is the last model to produce output, but it gets exclusive use of the screen
	if runModelView := o.runModel.View(); runModelView == "" {
		lines = append(lines, o.welcomeModel.View())
		lines = append(lines, o.generationModel.View())
		lines = append(lines, o.previewModel.View())
	} else {
		lines = append(lines, runModelView)
	}

	if o.maybeQuit {
		lines = append(lines, "\n", statusf("😢", "Are you sure you want to quit? [Y/n]: "))
	}

	if o.lastErr != nil {
		lines = append(lines, "\n", statusf("❌", " Error: %s", o.lastErr.Error()))

		// This information is usually to useful to not show
		eerr := &exec.ExitError{}
		if errors.As(o.lastErr, &eerr) {
			lines = append(lines, "\n", string(eerr.Stderr))
		}
	}

	return strings.Join(lines, "")
}

// View returns the rendering of the welcome model.
func (m welcomeModel) View() string {
	var lines []string

	if m.BuildVersion != "" {
		lines = append(lines, statusf("😄", "%s %s", m.CommandName, m.BuildVersion))
		if m.KubectlVersion != "" {
			lines = append(lines, statusf(" ", " ▪ kubectl %s", m.KubectlVersion))
			if m.ForgeVersion != "" && m.ForgeVersion != "unknown" {
				lines = append(lines, statusf(" ", " ▪ forge %s", m.ForgeVersion))
			}
		}
	}

	if m.BuildVersion == "" || m.KubectlVersion == "" || m.ForgeVersion == "" {
		return strings.Join(lines, "")
	}

	switch m.Authorization {
	case azValid:
		lines = append(lines, statusf("🗝", "Authorization found"))
	case azIgnored:
		lines = append(lines, statusf("🤷", "Continuing without authorization"))
	case azInvalid:
		lines = append(lines, "\n", "You are not logged in, are you sure you want to continue? [Y/n]: ")
	default:
		return strings.Join(lines, "")
	}

	if m.InitializationPercent > 0 {
		lines = append(lines, statusf("💾", "Initializing ..."))
		lines = append(lines, statusf("", " ▪ Using image %s", m.ControllerImage))
	}

	if m.ControllerVersion != "" && m.ControllerVersion != "unknown" {
		lines = append(lines, statusf("👍", "Running controller %s", m.ControllerVersion))
	}

	return strings.Join(lines, "")
}

// View returns the rendering of the generation model.
func (m generationModel) View() string {
	// Get the fields and the current field index
	fields := m.fields()
	idx := m.fieldIndex
	if idx < 0 {
		idx = len(fields) // If we wrapped around to -1, we are done, show all fields
	}

	var lines []string
	lines = append(lines, "")
	for i := 0; i < len(fields) && i < idx; i++ {
		lines = append(lines, fields[i].View())
	}

	return strings.Join(lines, "\n")
}

// View returns the rendering of the preview model.
func (m previewModel) View() string {
	if m.experiment == nil {
		return ""
	}

	var lines []string

	lines = append(lines, "", statusf("🎉", "Your experiment is ready to run!"))
	lines = append(lines, fmt.Sprintf("Name: %s", m.experiment.Name))

	lines = append(lines, fmt.Sprintf("Parameters:"))
	for i := range m.experiment.Spec.Parameters {
		p := &m.experiment.Spec.Parameters[i]
		lines = append(lines, fmt.Sprintf("  %s (from %d to %d)", p.Name, p.Min, p.Max))
	}

	lines = append(lines, fmt.Sprintf("Metrics:"))
	for i := range m.experiment.Spec.Metrics {
		m := &m.experiment.Spec.Metrics[i]
		if m.Optimize == nil || *m.Optimize {
			lines = append(lines, fmt.Sprintf("  %s", m.Name))
		}
	}

	lines = append(lines, "")

	if m.confirmed {
		lines = append(lines, statusf("🚢", "Starting experiment ..."))
	} else {
		lines = append(lines, "Ready to run? [Y/n]: ")
	}

	return strings.Join(lines, "\n")
}

// View returns the rendering of the run model.
func (m runModel) View() string {
	if m.trials == nil {
		return ""
	}

	if m.completed {
		return statusf("🍾", "Your experiment is complete!")
	} else if m.failed {
		return statusf("😫", "Your experiment failed.")
	} else if m.trialFailureCount > 10 {
		return statusf("😬", "This isn't going so well. Maybe try cleaning up the namespace and running again?")
	}

	var buf strings.Builder
	_, _ = fmt.Fprintln(&buf, statusf("👓", "Your experiment is running, hit ctrl-c to stop watching"))

	type column struct {
		name string
		path []string
	}
	columns := []column{
		{name: "NAME", path: []string{"metadata", "name"}},
		{name: "STATUS", path: []string{"status", "phase"}},
		{name: "ASSIGNMENTS", path: []string{"status", "assignments"}},
		{name: "VALUES", path: []string{"status", "values"}},
	}

	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
	printCol := func(v string, c int) {
		_, _ = w.Write([]byte(v))
		if c < len(columns)-1 {
			_, _ = w.Write([]byte{'\t'})
		} else {
			_, _ = w.Write([]byte{'\n'})
		}
	}

	for c := range columns {
		printCol(columns[c].name, c)
	}

	for _, node := range m.trials {
		for c := range columns {
			v, err := node.Pipe(yaml.Lookup(columns[c].path...))
			if err != nil {
				return err.Error() // TODO ???
			}
			printCol(v.YNode().Value, c)
		}
	}

	if err := w.Flush(); err != nil {
		return err.Error() // TODO ???
	}

	return buf.String()
}

// statusf is used for format a single line status message.
func statusf(icon, message string, args ...interface{}) string {
	return fmt.Sprintf("%-2s "+message+"\n", append([]interface{}{icon}, args...)...)
}

// input is a helper interface to help with sequential fields.
type input interface {
	Focus()
	Focused() bool
	Blur()
	View() string
}

// requiredInput is an input which may refuse to give up focus if it is required.
type requiredInput interface {
	input
	TryBlur() bool
}

func focusNext(inputs []input, idx int) int {
	// The cycle is complete or there are no fields
	if idx <= 0 || len(inputs) == 0 {
		return idx
	}

	if r, ok := inputs[idx-1].(requiredInput); ok {
		if !r.TryBlur() {
			return idx
		}
	} else {
		inputs[idx-1].Blur()
	}

	if idx >= len(inputs) {
		return -1
	}

	inputs[idx].Focus()
	return idx + 1
}

func isLastFocused(inputs []input) bool {
	if len(inputs) == 0 {
		return false
	}
	return inputs[len(inputs)-1].Focused()
}
