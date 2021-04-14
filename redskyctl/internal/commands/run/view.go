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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/run/form"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// initializeModel is invoked before the program is started to ensure things
// are in a valid state prior to starting.
func (o *Options) initializeModel() {

	o.generationModel.StormForgerTestCaseInput = form.NewChoiceField()
	o.generationModel.StormForgerTestCaseInput.Prompt = promptf("Please select a StormForger test case to optimize for:", long)
	o.generationModel.StormForgerTestCaseInput.LoadingMessage = " Fetching test cases from StormForger ..."
	o.generationModel.StormForgerTestCaseInput.Instructions = "\nup/down: select  |  enter: continue"

	o.generationModel.LocustfileInput = form.NewTextField()
	o.generationModel.LocustfileInput.Prompt = promptf("Enter the location of the locustfile.py you would like to run:", short)
	o.generationModel.LocustfileInput.Completions = &form.FileCompletions{Extensions: []string{".py"}}
	o.generationModel.LocustfileInput.Validator = &form.File{Required: "Required", Missing: "File does not exist"}
	o.generationModel.LocustfileInput.Enable()

	o.generationModel.NamespaceInput = form.NewMultiChoiceField()
	o.generationModel.NamespaceInput.Prompt = promptf("Please select the Kubernetes namespace where your application is running:", long)
	o.generationModel.NamespaceInput.LoadingMessage = " Fetching namespaces from Kubernetes ..."
	o.generationModel.NamespaceInput.Instructions = "\nup/down: select  |  space: choose  |  enter: continue"
	o.generationModel.NamespaceInput.Validator = &form.Required{Error: "Required"}

	o.generationModel.LabelSelectorTemplate = func(namespace string) form.TextField {
		labelSelectorInput := form.NewTextField()
		labelSelectorInput.Prompt = promptf("Specify the label selector for your application resources in the '%s' namespace:", long, namespace)
		labelSelectorInput.Placeholder = "All resources"
		labelSelectorInput.Instructions = "Leave blank to select all resources"
		labelSelectorInput.Validator = &labelSelectorValidator{InvalidSelector: "Must be a valid label selector"}
		return labelSelectorInput
	}

	o.generationModel.IngressURLInput = form.NewTextField()
	o.generationModel.IngressURLInput.Prompt = promptf("Enter the URL of the endpoint to test:", short)
	o.generationModel.IngressURLInput.Validator = &form.URL{Required: "Required", InvalidURL: "Must be a valid URL", Absolute: "URL must be absolute"}
	o.generationModel.IngressURLInput.Enable()

	o.generationModel.ContainerResourcesSelectorInput = form.NewTextField()
	o.generationModel.ContainerResourcesSelectorInput.Prompt = promptf("Specify the label selector matching resources which should have their memory and CPU optimized:", long)
	o.generationModel.ContainerResourcesSelectorInput.Placeholder = "All resources"
	o.generationModel.ContainerResourcesSelectorInput.Instructions = "Leave blank to select all resources"
	o.generationModel.ContainerResourcesSelectorInput.Validator = &labelSelectorValidator{InvalidSelector: "Must be a valid label selector"}
	o.generationModel.ContainerResourcesSelectorInput.Enable()

	o.generationModel.ReplicasSelectorInput = form.NewTextField()
	o.generationModel.ReplicasSelectorInput.Prompt = promptf("Specify the label selector matching resources which can be scaled horizontally:", long)
	o.generationModel.ReplicasSelectorInput.Placeholder = "No resources"
	o.generationModel.ReplicasSelectorInput.Instructions = "Must be a valid Kubernetes label selector, leave blank to select no resources"
	o.generationModel.ReplicasSelectorInput.Validator = &labelSelectorValidator{InvalidSelector: "Must be a valid label selector"}
	o.generationModel.ReplicasSelectorInput.Enable()

	o.generationModel.ObjectiveInput = form.NewMultiChoiceField()
	o.generationModel.ObjectiveInput.Prompt = promptf("Please select objectives to optimize:", long)
	o.generationModel.ObjectiveInput.Instructions = "\nup/down: select  |  space: choose  |  enter: continue"
	o.generationModel.ObjectiveInput.Choices = []string{
		"cost",
		"p50-latency",
		"p95-latency",
		"p99-latency",
	}
	o.generationModel.ObjectiveInput.Select(0)
	o.generationModel.ObjectiveInput.Select(2)
	o.generationModel.ObjectiveInput.Enable()

}

// View returns a full rendering of the current state. This method is called
// from the event loop and must not block, it must return as fast as possible.
func (o *Options) View() string {
	var lines []string

	// The "runModel" is the last model to produce output, but it gets exclusive use of the screen
	if o.status != "" {
		lines = append(lines, o.status)
	} else if runModelView := o.runModel.View(); runModelView == "" {
		lines = append(lines, o.initializationModel.View())
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

		// This information is usually too useful to not show
		eerr := &exec.ExitError{}
		if errors.As(o.lastErr, &eerr) {
			lines = append(lines, "\n", string(eerr.Stderr))
		}
	}

	return strings.Join(lines, "")
}

// View returns the rendering of the welcome model. To ensure consistent output,
// this must render progressively as information comes back from the asynchronous
// initialization commands (e.g. even if the forge version comes back before the
// kubectl version, we still always render kubectl first, stopping the progression
// if it isn't available yet).
func (m initializationModel) View() string {
	var lines []string

	if m.BuildVersion != "" {
		lines = append(lines, statusf("😄", "%s %s", m.CommandName, m.BuildVersion))
	} else {
		return strings.Join(lines, "")
	}

	if m.KubectlVersion != "" {
		if m.KubectlVersion != unknownVersion {
			lines = append(lines, statusf(" ", " ▪ kubectl %s", m.KubectlVersion))
		}
	} else {
		return strings.Join(lines, "")
	}

	if m.ForgeVersion != "" {
		if m.ForgeVersion != unknownVersion {
			lines = append(lines, statusf(" ", " ▪ forge %s", m.ForgeVersion))
		}
	} else {
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

	if m.InitializationPercent > 0 && m.InitializationPercent < 1 {
		lines = append(lines, statusf("💾", "Initializing ..."))
		lines = append(lines, statusf("", " ▪ Using image %s", m.ControllerImage))
		// TODO Progress bar based on the initialization percentage
	}

	if m.ControllerVersion != "" && m.ControllerVersion != "unknown" {
		lines = append(lines, statusf("👍", "Running controller %s", m.ControllerVersion))
	}

	return strings.Join(lines, "")
}

// View returns the rendering of the generation model.
func (m generationModel) View() string {
	return m.form().View()
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

type promptStyle int

const (
	// short is single line prompt.
	short promptStyle = 0
	// long is a multi-line prompt.
	long promptStyle = 1
)

// promptf is used to format an input prompt.
func promptf(message string, style promptStyle, args ...interface{}) string {
	switch style {
	case short:
		return fmt.Sprintf("\n"+message+" ", args...)
	case long:
		return fmt.Sprintf("\n"+message+"\n", args...)
	default:
		panic("unknown prompt style")
	}
}

// statusf is used for format a single line status message.
func statusf(icon, message string, args ...interface{}) string {
	return fmt.Sprintf("%-2s "+message+"\n", append([]interface{}{icon}, args...)...)
}

// labelSelectorValidator ensures a text field represents a valid Kubernetes label selector.
type labelSelectorValidator struct {
	InvalidSelector string
}

func (v labelSelectorValidator) ValidateTextField(value string) tea.Msg {
	if _, err := labels.Parse(value); err != nil {
		return form.ValidationMsg(v.InvalidSelector)
	}
	return form.ValidationMsg("")
}
