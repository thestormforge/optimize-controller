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
	"text/tabwriter"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/run/form"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/run/out"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// initializeModel is invoked before the program is started to ensure things
// are in a valid state prior to starting.
func (o *Options) initializeModel() {
	var opts []out.FieldOption
	if o.Verbose {
		opts = append(opts, out.VerbosePrompts)
	}

	o.generationModel.NamespaceInput = out.FormField{
		Prompt:         "Please select the Kubernetes namespace(s) where your application is running:",
		LoadingMessage: "Fetching namespaces from Kubernetes",
		Instructions:   []string{"up/down: select", "x: choose", "enter: continue"},
	}.NewMultiChoiceField(opts...)
	o.generationModel.NamespaceInput.Validator = &form.Required{
		Error: "Required",
	}

	o.generationModel.LabelSelectorTemplate = func(namespace string) form.TextField {
		labelSelectorInput := out.FormField{
			Prompt:       fmt.Sprintf("Specify the label selector for your application resources in the '%s' namespace:", namespace),
			Placeholder:  "All resources",
			Instructions: []string{"Leave blank to select all resources"},
		}.NewTextField(opts...)
		labelSelectorInput.Validator = &labelSelectorValidator{
			InvalidSelector: "Must be a valid label selector",
		}
		return labelSelectorInput
	}

	o.generationModel.StormForgerTestCaseInput = out.FormField{
		Prompt:         "Please select a StormForger test case to optimize for:",
		PromptVerbose:  "This is an example of a more verbose prompt.\nSelect a StormForger test case:",
		LoadingMessage: "Fetching test cases from StormForger",
		Instructions:   []string{"up/down: select", "enter: continue"},
	}.NewChoiceField(opts...)

	o.generationModel.LocustfileInput = out.FormField{
		Prompt:          "Enter the location of the locustfile.py you would like to run:",
		InputOnSameLine: true,
		Enabled:         true,
		Completions: &form.FileCompletions{
			Extensions: []string{".py"},
		},
	}.NewTextField(opts...)
	o.generationModel.LocustfileInput.Validator = &form.File{
		Required:    "Required",
		Missing:     "File does not exist",
		RegularFile: "Must be a file, not a directory",
	}

	o.generationModel.IngressURLInput = out.FormField{
		Prompt:          "Enter the URL of the endpoint to test:",
		InputOnSameLine: true,
		Enabled:         true,
	}.NewTextField(opts...)
	o.generationModel.IngressURLInput.Validator = &form.URL{
		Required:   "Required",
		InvalidURL: "Must be a valid URL",
		Absolute:   "URL must be absolute",
	}

	o.generationModel.ContainerResourcesSelectorInput = out.FormField{
		Prompt:       "Specify the label selector matching resources which should have their memory and CPU optimized:",
		Placeholder:  "All resources",
		Instructions: []string{"Leave blank to select all resources"},
		Enabled:      true,
	}.NewTextField(opts...)
	o.generationModel.ContainerResourcesSelectorInput.Validator = &labelSelectorValidator{
		InvalidSelector: "Must be a valid label selector",
	}

	o.generationModel.ReplicasSelectorInput = out.FormField{
		Prompt:       "Specify the label selector matching resources which can be scaled horizontally:",
		Placeholder:  "No resources",
		Instructions: []string{"Must be a valid Kubernetes label selector, leave blank to select no resources"},
		Enabled:      true,
	}.NewTextField(opts...)
	o.generationModel.ReplicasSelectorInput.Validator = &labelSelectorValidator{
		InvalidSelector: "Must be a valid label selector",
	}

	o.generationModel.ObjectiveInput = out.FormField{
		Prompt:       "Please select objectives to optimize:",
		Instructions: []string{"up/down: select", "x: choose", "enter: continue"},
		Enabled:      true,
		Choices: []string{
			"cost",
			"p50-latency",
			"p95-latency",
			"p99-latency",
		},
	}.NewMultiChoiceField(opts...)
	o.generationModel.ObjectiveInput.Select(0)
	o.generationModel.ObjectiveInput.Select(2)

}

// View returns a full rendering of the current state. This method is called
// from the event loop and must not block, it must return as fast as possible.
func (o *Options) View() string {
	var view out.View
	switch {
	case o.status != "":
		// Special case for debugging
		_, _ = view.Write([]byte(o.status))

	case o.runModel.trials != nil:
		// Once the run model has trials, it gets exclusive use of the screen
		view.Model(o.runModel)

	default:
		// Otherwise combine the output of all the children models
		view.Model(o.initializationModel)
		view.Model(o.generationModel)
		view.Model(o.previewModel)
	}

	if o.maybeQuit {
		view.Newline()
		view.Step(out.Sad, "Are you sure you want to quit? [Y/n]: ")
	}

	if o.lastErr != nil {
		view.Newline()
		view.Step(out.Failure, "Error: %s", o.lastErr)

		// This information is usually too useful to not show
		eerr := &exec.ExitError{}
		if errors.As(o.lastErr, &eerr) {
			view.Newline()
			_, _ = view.Write(eerr.Stderr)
		}
	}

	return view.String()
}

// View returns the rendering of the welcome model. To ensure consistent output,
// this must render progressively as information comes back from the asynchronous
// initialization commands (e.g. even if the forge version comes back before the
// kubectl version, we still always render kubectl first, stopping the progression
// if it isn't available yet).
func (m initializationModel) View() string {
	var view out.View

	if m.BuildVersion != "" {
		view.Step(out.Happy, "%s %s", m.CommandName, m.BuildVersion)
	} else {
		return view.String()
	}

	if m.KubectlVersion != "" {
		if m.KubectlVersion != unknownVersion {
			view.Step(out.Version, "kubectl %s", m.KubectlVersion)
		}
	} else {
		return view.String()
	}

	if m.ForgeVersion != "" {
		if m.ForgeVersion != unknownVersion {
			view.Step(out.Version, "forge %s", m.ForgeVersion)
		}
	} else {
		return view.String()
	}

	switch m.Authorization {
	case azValid:
		view.Step(out.Authorized, "Authorization found")
	case azIgnored:
		view.Step(out.Unauthorized, "Continuing without authorization")
	case azInvalid:
		view.Newline()
		view.Step(out.YesNo, "You are not logged in, are you sure you want to continue? [Y/n]: ")
	default:
		return view.String()
	}

	if m.InitializationPercent > 0 && m.InitializationPercent < 1 {
		view.Step(out.Initializing, "Initializing ...")
		view.Step(out.Version, "Using image %s", m.ControllerImage)
		// TODO Progress bar based on the initialization percentage
	}

	if m.ControllerVersion != "" && m.ControllerVersion != "unknown" {
		view.Step(out.Running, "Running controller %s", m.ControllerVersion)
	}

	return view.String()
}

// View returns the rendering of the generation model.
func (m generationModel) View() string {
	return m.form().View()
}

// View returns the rendering of the preview model.
func (m previewModel) View() string {
	var view out.View
	if m.experiment == nil {
		return view.String()
	}

	view.Newline()
	view.Step(out.Ready, "Your experiment is ready to run!")

	view.Newline()
	view.Step(out.Preview, "Name: %s", m.experiment.Name)
	view.Step(out.Preview, "Parameters:")
	for i := range m.experiment.Spec.Parameters {
		p := &m.experiment.Spec.Parameters[i]
		view.Step(out.Preview, "  %s (from %d to %d)", p.Name, p.Min, p.Max)
	}
	view.Step(out.Preview, "Metrics:")
	for i := range m.experiment.Spec.Metrics {
		m := &m.experiment.Spec.Metrics[i]
		if m.Optimize == nil || *m.Optimize {
			view.Step(out.Preview, "  %s", m.Name)
		}
	}

	view.Newline()
	if m.confirmed {
		view.Step(out.Starting, "Starting experiment ...")
	} else {
		view.Step(out.YesNo, "Ready to run? [Y/n]: ")
	}

	return view.String()
}

// View returns the rendering of the run model.
func (m runModel) View() string {
	var view out.View
	if m.trials == nil {
		return view.String()
	}

	if m.completed {
		view.Step(out.Completed, "Your experiment is complete!")
		return view.String()
	} else if m.failed {
		view.Step(out.ReallySad, "Your experiment failed.")
		return view.String()
	} else if m.trialFailureCount > 10 {
		view.Step(out.NotGood, "This isn't going so well. Maybe try cleaning up the namespace and running again?")
		return view.String()
	}

	view.Step(out.Watching, "Your experiment is running, hit ctrl-c to stop watching")
	view.Newline()

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

	w := tabwriter.NewWriter(&view, 0, 0, 3, ' ', 0)
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

	return view.String()
}

// labelSelectorValidator ensures a text field represents a valid Kubernetes label selector.
type labelSelectorValidator struct {
	InvalidSelector string
}

func (v labelSelectorValidator) ValidateTextField(value string) tea.Msg {
	if _, err := labels.Parse(value); err != nil {
		msg := v.InvalidSelector
		if msg == "" {
			msg = err.Error()
		}
		return form.ValidationMsg(msg)
	}
	return form.ValidationMsg("")
}
