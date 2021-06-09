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
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/run/form"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/run/internal"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/run/out"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/run/pager"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	ScenarioTypeStormForger = "StormForge"
	ScenarioTypeLocust      = "Locust"

	DestinationCreate  = "Run the experiment"
	DestinationFile    = "Save the experiment to disk"
	DestinationPreview = "Inspect the experiment"
	// TODO DestinationDelete = "Clean up a previous run"?
)

// initializeModel is invoked before the program is started to ensure things
// are in a valid state prior to starting.
func (o *Options) initializeModel() {
	var opts []out.FieldOption
	if o.Verbose {
		opts = append(opts, out.VerbosePrompts)
	}
	opts = append(opts, out.GlobalInstructions(
		out.KeyBinding{
			Key:  tea.Key{Type: tea.KeyPgUp},
			Desc: "back",
		},
		out.KeyBinding{
			Key:  tea.Key{Type: tea.KeyEnter},
			Desc: "continue",
		},
	))

	o.generatorModel.ScenarioType = out.FormField{
		Prompt: "Where do you want to get your load test from?",
		Instructions: []interface{}{
			"up/down: select",
		},
		Choices: []string{
			ScenarioTypeStormForger,
			ScenarioTypeLocust,
		},
	}.NewChoiceField(opts...) // TODO This includes the "back" instruction even though it's not possible
	o.generatorModel.ScenarioType.Select(0)

	o.generatorModel.StormForgerTestCaseInput = out.FormField{
		Prompt:         "Please select a load test to optimize:",
		LoadingMessage: "Fetching test cases from StormForger",
		Instructions: []interface{}{
			"up/down: select",
		},
	}.NewChoiceField(opts...)

	o.generatorModel.StormForgerGettingStarted = out.FormField{
		Prompt: `Check this out to see how you can get set up with a StormForge load test:
https://docs.stormforger.com/guides/getting-started/`,
	}.NewExitField(opts...)

	o.generatorModel.LocustfileInput = out.FormField{
		Prompt:          "Please input a path to your Locust load test to optimize:",
		Placeholder:     "( e.g. ~/my-project/tests/locustfile.py )",
		InputOnSameLine: true,
		Completions: &form.FileCompletions{
			Extensions: []string{".py"},
		},
	}.NewTextField(opts...)
	o.generatorModel.LocustfileInput.Validator = &form.File{
		Required:    "Required",
		Missing:     "File does not exist",
		RegularFile: "Must be a file, not a directory",
	}

	o.generatorModel.NamespaceInput = out.FormField{
		Prompt:         "Please select the namespace(s) where your application is running:",
		LoadingMessage: "Fetching namespaces from Kubernetes",
		Instructions: []interface{}{
			"up/down: select",
			out.KeyBinding{Key: tea.Key{Type: tea.KeyRunes, Runes: []rune{'x'}}, Desc: "choose"},
		},
	}.NewMultiChoiceField(opts...)
	o.generatorModel.NamespaceInput.Validator = &form.Required{
		Error: "Required",
	}

	o.generatorModel.LabelSelectorTemplate = func(namespace string) form.TextField {
		labelSelectorInput := out.FormField{
			Prompt:      fmt.Sprintf("Specify labels for '%s' namespace:", namespace),
			Placeholder: "( e.g. environment=dev, tier=frontend )",
			Instructions: []interface{}{
				"Leave blank to select all resources",
			},
		}.NewTextField(opts...)
		labelSelectorInput.Validator = &labelSelectorValidator{
			InvalidSelector: "Must be a valid label selector",
		}
		return labelSelectorInput
	}

	o.generatorModel.IngressURLInput = out.FormField{
		Prompt:          "Enter the URL of the endpoint to test:",
		Placeholder:     "( e.g. http://my-app.svc.cluster.local )",
		InputOnSameLine: true,
		Completions:     form.StaticCompletions{"http://", "https://"},
	}.NewTextField(opts...)
	o.generatorModel.IngressURLInput.Validator = &form.URL{
		Required:   "Required",
		InvalidURL: "Must be a valid URL",
		Absolute:   "URL must be absolute",
	}

	o.generatorModel.ContainerResourcesSelectorInput = out.FormField{
		Prompt:      "Specify labels to control discovery of memory and CPU parameters:",
		Placeholder: "( e.g. component=api )",
		Instructions: []interface{}{
			"Leave blank to select all resources",
		},
	}.NewTextField(opts...)
	o.generatorModel.ContainerResourcesSelectorInput.Validator = &labelSelectorValidator{
		InvalidSelector: "Must be a valid label selector",
	}

	o.generatorModel.ReplicasSelectorInput = out.FormField{
		Prompt:      "Specify labels to control discovery of replica parameters:",
		Placeholder: "( e.g. component=api )",
		Instructions: []interface{}{
			"Leave blank to select no resources",
		},
	}.NewTextField(opts...)
	o.generatorModel.ReplicasSelectorInput.Validator = &labelSelectorValidator{
		InvalidSelector: "Must be a valid label selector",
	}

	o.generatorModel.ObjectiveInput = out.FormField{
		Prompt: "Please select objectives to optimize:",
		Instructions: []interface{}{
			"up/down: select",
			out.KeyBinding{Key: tea.Key{Type: tea.KeyRunes, Runes: []rune{'x'}}, Desc: "choose"},
		},
		Choices: []string{
			"cost",
			"p50-latency",
			"p95-latency",
			"p99-latency",
		},
	}.NewMultiChoiceField(opts...)
	o.generatorModel.ObjectiveInput.Select(0)
	o.generatorModel.ObjectiveInput.Select(2)

	o.previewModel.Destination = out.FormField{
		Prompt: "What would you like to do?",
		Choices: []string{
			DestinationCreate,
			DestinationPreview,
			DestinationFile,
		},
	}.NewChoiceField(opts...) // TODO This includes the "back" instruction even though it's not possible
	o.previewModel.Destination.Select(0)

	o.previewModel.Filename = out.FormField{
		Prompt:          "\nEnter the path where you would like to save your experiment:",
		Placeholder:     "( e.g. ~/my-project/experiment.yaml )",
		InputOnSameLine: true,
		Completions:     &form.FileCompletions{},
	}.NewTextField(opts...)
	o.previewModel.Filename.Validator = &form.File{
		Required: "Required",
	}

	o.previewModel.Preview = pager.NewModel()
	o.previewModel.Preview.Instructions = out.PagerInstructions([]out.KeyBinding{
		{
			Key:  tea.Key{Type: tea.KeyCtrlX},
			Desc: "Quit",
		},
		{
			Key:  tea.Key{Type: tea.KeySpace},
			Desc: "Next page",
		},
		{
			Key:  tea.Key{Type: tea.KeyRunes, Runes: []rune{'b'}},
			Desc: "Previous page",
		},
	})

}

// View returns a full rendering of the current state. This method is called
// from the event loop and must not block, it must return as fast as possible.
func (o *Options) View() string {
	var view out.View
	switch {

	case o.runModel.trials != nil:
		// Once the run model has trials, it gets exclusive use of the screen
		view.Model(o.runModel)

	default:
		// Otherwise combine the output of all the children models
		view.Model(o.initializationModel)
		view.Model(o.generatorModel.form())
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

	view.Step(out.Happy, "%s %s", m.CommandName, m.BuildVersion)

	if m.KubectlVersion == nil {
		return view.String()
	}

	if m.KubectlVersion.Available() {
		view.Step(out.Version, "kubectl %s", m.KubectlVersion)
	}

	if m.ForgeVersion == nil {
		return view.String()
	}

	if m.ForgeVersion.Available() {
		view.Step(out.Version, "forge %s", m.ForgeVersion)
	}

	switch m.OptimizeAuthorization {
	case internal.AuthorizationValid:
		view.Step(out.Authorized, "StormForge Optimize authorization found")
	case internal.AuthorizationInvalidIgnored:
		view.Step(out.Unauthorized, "Continuing without StormForge Optimize authorization")
	case internal.AuthorizationInvalid:
		view.Newline()
		view.Step(out.YesNo, "You are not logged in, are you sure you want to continue? [Y/n]: ")
		return view.String()
	default:
		return view.String()
	}

	switch m.PerformanceTestAuthorization {
	case internal.AuthorizationValid:
		// Do nothing
	case internal.AuthorizationInvalidIgnored:
		view.Step(out.Unauthorized, "StormForge Performance Test authorization not found")
	case internal.AuthorizationUnknown:
		return view.String()
	}

	if m.InitializeCluster {
		view.Step(out.Initializing, "Initializing ...")
		view.Step(out.Version, "Using image %s", m.ControllerImage)
	}

	if m.ControllerVersion.Available() {
		view.Step(out.Running, "Running controller %s", m.ControllerVersion)
	}

	if m.Done() {
		view.Newline()
		view.Step(out.Preview, "Welcome to StormForge!")
	}

	return view.String()
}

func (o *Options) updateGeneratorForm() {
	if !o.initializationModel.Done() {
		return
	}

	if len(o.Generator.Application.Scenarios) == 0 {
		o.generatorModel.ScenarioType.Enable()
		useStormForger := o.generatorModel.ScenarioType.Value() == ScenarioTypeStormForger
		useLocust := o.generatorModel.ScenarioType.Value() == ScenarioTypeLocust

		forgeAvailable := o.initializationModel.ForgeVersion.Available() &&
			o.initializationModel.PerformanceTestAuthorization == internal.AuthorizationValid
		o.generatorModel.StormForgerTestCaseInput.SetEnabled(useStormForger && forgeAvailable)
		o.generatorModel.StormForgerGettingStarted.SetEnabled(useStormForger && !forgeAvailable)

		o.generatorModel.LocustfileInput.SetEnabled(useLocust)
		o.generatorModel.IngressURLInput.SetEnabled(useLocust)
	}

	if len(o.Generator.Application.Resources) == 0 {
		kubectlAvailable := o.initializationModel.KubectlVersion.Available()
		o.generatorModel.NamespaceInput.SetEnabled(kubectlAvailable)
	}

	if o.Generator.Application.Parameters == nil { // Parameters can be defaulted so allow an empty list
		o.generatorModel.ContainerResourcesSelectorInput.Enable()
		o.generatorModel.ReplicasSelectorInput.Enable()
	}

	if len(o.Generator.Application.Objectives) == 0 {
		o.generatorModel.ObjectiveInput.Enable()
	}
}

// View returns the rendering of the preview model.
func (m previewModel) View() string {
	var view out.View
	if m.Experiment == nil {
		return view.String()
	}

	view.Newline()
	view.Step(out.Ready, "Your experiment is ready to run!")

	view.Newline()
	view.Step(out.Preview, "Application Name: %s", m.Experiment.Labels[optimizeappsv1alpha1.LabelApplication])
	view.Step(out.Preview, "Experiment Name: %s", m.Experiment.Name)
	view.Step(out.Preview, "Parameters:")
	for i := range m.Experiment.Spec.Parameters {
		p := &m.Experiment.Spec.Parameters[i]
		view.Step(out.Preview, "  %s (from %d to %d)", p.Name, p.Min, p.Max)
	}
	view.Step(out.Preview, "Metrics:")
	for i := range m.Experiment.Spec.Metrics {
		m := &m.Experiment.Spec.Metrics[i]
		if m.Optimize == nil || *m.Optimize {
			view.Step(out.Preview, "  %s", m.Name)
		}
	}

	view.Newline()
	view.Model(m.Destination)
	view.Model(m.Filename)
	view.Newline()
	view.Model(m.Preview)
	if m.Create {
		view.Newline()
		view.Step(out.Starting, "Starting experiment ...")
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
