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
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/scan"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/run/form"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/run/internal"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// initializationModel is used at the beginning of the run command to print
// informational messages and initialize the controller.
type initializationModel struct {
	// Name of the tool, as invoked by the user.
	CommandName string
	// Version of the tool, as embedded by the build.
	BuildVersion string
	// Image name of the controller to use for initialization.
	ControllerImage string

	// Version of the forge command.
	ForgeVersion *internal.Version
	// Version of the kubectl command.
	KubectlVersion *internal.Version
	// Version of the currently running controller.
	ControllerVersion *internal.Version

	// Optimize authorization status.
	OptimizeAuthorization internal.AuthorizationStatus
	// Performance test authorization status.
	PerformanceTestAuthorization internal.AuthorizationStatus

	// Flag indicating that we must perform cluster initialization.
	InitializeCluster bool
}

// Done returns true if the initialization model has reached a state where it has
// not pending information left to collect and display.
func (m initializationModel) Done() bool {
	return m.ForgeVersion != nil &&
		m.KubectlVersion != nil &&
		m.ControllerVersion.Available() &&
		m.OptimizeAuthorization.Allowed() &&
		m.PerformanceTestAuthorization.Allowed()
}

// Update returns a copy of the model after applying the supplied message. If
// any further action is required, the returned command will be non-nil.
func (m initializationModel) Update(msg tea.Msg) (initializationModel, tea.Cmd) {
	// Check if we are in a "done" state before the update so we can detect the change
	done := m.Done()

	var cmds []tea.Cmd
	switch msg := msg.(type) {

	case internal.ForgeVersionMsg:
		m.ForgeVersion = internal.NewVersion(msg)

	case internal.KubectlVersionMsg:
		m.KubectlVersion = internal.NewVersion(msg)

	case internal.OptimizeControllerVersionMsg:
		m.ControllerVersion = internal.NewVersion(msg)

		// Record that the controller is not available so we can show the initialization message
		if !m.ControllerVersion.Available() {
			m.InitializeCluster = true
		}

	case internal.OptimizeAuthorizationMsg:
		m.OptimizeAuthorization = internal.AuthorizationStatus(msg)

	case internal.PerformanceTestAuthorizationMsg:
		m.PerformanceTestAuthorization = internal.AuthorizationStatus(msg)

		// Ignore failed performance test authorization without asking the user
		if m.PerformanceTestAuthorization == internal.AuthorizationInvalid {
			cmds = append(cmds, func() tea.Msg { return internal.PerformanceTestAuthorizationMsg(internal.AuthorizationInvalidIgnored) })
		}

	case tea.KeyMsg:
		// If the authorization is invalid, check to see if the user wants to ignore it
		if m.OptimizeAuthorization == internal.AuthorizationInvalid {
			switch msg.String() {
			case "y", "Y", "enter":
				cmds = append(cmds, func() tea.Msg { return internal.OptimizeAuthorizationMsg(internal.AuthorizationInvalidIgnored) })
			case "n", "N":
				// TODO Return an error instead so we can say something like "You should run redskyctl login"?
				return m, tea.Quit
			}
		}

	}

	// If this update changed the "done" status, start the form input
	if !done && m.Done() {
		cmds = append(cmds, form.Start)
	}

	return m, tea.Batch(cmds...)
}

// generatorModel holds the inputs for values on the generator.
type generatorModel struct {
	ScenarioType              form.ChoiceField
	StormForgerTestCaseInput  form.ChoiceField
	StormForgerGettingStarted form.ExitField
	LocustfileInput           form.TextField

	NamespaceInput        form.MultiChoiceField
	LabelSelectorInputs   []form.TextField
	LabelSelectorTemplate func(namespace string) form.TextField

	IngressURLInput form.TextField

	ContainerResourcesSelectorInput form.TextField
	ReplicasSelectorInput           form.TextField

	ObjectiveInput form.MultiChoiceField
}

func (m generatorModel) Update(msg tea.Msg) (generatorModel, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {

	case internal.StormForgerTestCasesMsg:
		m.StormForgerTestCaseInput.Choices = msg
		m.StormForgerTestCaseInput.SelectOnly()

	case internal.KubernetesNamespacesMsg:
		m.NamespaceInput.Choices = msg
		m.NamespaceInput.SelectOnly()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			// If we just completed namespace selection, create the per-namespace label selector inputs
			if m.NamespaceInput.Focused() {
				m.updateLabelSelectorInputs()
			}
		}

	}

	var cmd tea.Cmd

	cmd = m.form().Update(msg)
	cmds = append(cmds, cmd)

	m.ScenarioType, cmd = m.ScenarioType.Update(msg)
	cmds = append(cmds, cmd)

	m.StormForgerTestCaseInput, cmd = m.StormForgerTestCaseInput.Update(msg)
	cmds = append(cmds, cmd)

	m.StormForgerGettingStarted, cmd = m.StormForgerGettingStarted.Update(msg)
	cmds = append(cmds, cmd)

	m.LocustfileInput, cmd = m.LocustfileInput.Update(msg)
	cmds = append(cmds, cmd)

	m.NamespaceInput, cmd = m.NamespaceInput.Update(msg)
	cmds = append(cmds, cmd)

	for i := range m.LabelSelectorInputs {
		m.LabelSelectorInputs[i], cmd = m.LabelSelectorInputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}

	m.IngressURLInput, cmd = m.IngressURLInput.Update(msg)
	cmds = append(cmds, cmd)

	m.ContainerResourcesSelectorInput, cmd = m.ContainerResourcesSelectorInput.Update(msg)
	cmds = append(cmds, cmd)

	m.ReplicasSelectorInput, cmd = m.ReplicasSelectorInput.Update(msg)
	cmds = append(cmds, cmd)

	m.ObjectiveInput, cmd = m.ObjectiveInput.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// form returns a slice of everything on the model that implements `form.Field`.
func (m *generatorModel) form() form.Fields {
	var fields form.Fields
	fields = append(fields, &m.ScenarioType)
	fields = append(fields, &m.StormForgerTestCaseInput)
	fields = append(fields, &m.StormForgerGettingStarted)
	fields = append(fields, &m.LocustfileInput)
	fields = append(fields, &m.NamespaceInput)
	for i := range m.LabelSelectorInputs {
		fields = append(fields, &m.LabelSelectorInputs[i])
	}
	fields = append(fields, &m.IngressURLInput)
	fields = append(fields, &m.ContainerResourcesSelectorInput)
	fields = append(fields, &m.ReplicasSelectorInput)
	fields = append(fields, &m.ObjectiveInput)
	return fields
}

func (m *generatorModel) updateLabelSelectorInputs() {
	// Get the current list of selected namespaces and create label selector inputs for each one
	namespaces := m.NamespaceInput.Values()
	labelSelectorInputs := make([]form.TextField, len(namespaces))
	for i, namespace := range namespaces {
		labelSelectorInputs[i] = m.LabelSelectorTemplate(namespace)
		labelSelectorInputs[i].Enable()
	}

	// Preserve the values if someone went backwards in the form
	if len(m.LabelSelectorInputs) == len(labelSelectorInputs) {
		for i := range labelSelectorInputs {
			if m.LabelSelectorInputs[i].Prompt == labelSelectorInputs[i].Prompt {
				labelSelectorInputs[i].SetValue(m.LabelSelectorInputs[i].Value())
				labelSelectorInputs[i].CursorEnd()
			}
		}
	}
	m.LabelSelectorInputs = labelSelectorInputs
}

type previewModel struct {
	Experiment *redskyv1beta1.Experiment
	Confirmed  bool
}

func (m previewModel) Update(msg tea.Msg) (previewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case internal.ExperimentMsg:
		// Extract the experiment definition from the YAML to make it easier to pull values from
		obj := scan.ObjectList{}
		if err := obj.Write(msg); err != nil {
			return m, internal.Error(err)
		}

		for i := range obj.Items {
			if exp, ok := obj.Items[i].Object.(*redskyv1beta1.Experiment); ok {
				m.Experiment = exp
			}
		}

	case internal.ExperimentConfirmedMsg:
		// Update our confirmed state
		m.Confirmed = true

	case tea.KeyMsg:
		// Prompt the user to see if they want to run the experiment
		if m.Experiment != nil && !m.Confirmed {
			switch msg.String() {
			case "y", "Y", "enter":
				return m, func() tea.Msg { return internal.ExperimentConfirmedMsg{} }

			case "n", "N":
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

type runModel struct {
	experiment kio.ResourceNodeSlice
	trials     kio.ResourceNodeSlice

	completed         bool
	failed            bool
	trialFailureCount int
}

func (m runModel) Update(msg tea.Msg) (runModel, tea.Cmd) {
	switch msg := msg.(type) {

	case internal.ExperimentMsg:
		m.experiment = kio.ResourceNodeSlice(msg)

	case internal.TrialsMsg:
		m.trials = kio.ResourceNodeSlice(msg)

	case internal.ExperimentFinishedMsg:
		m.completed = !msg.Failed
		m.failed = msg.Failed
		return m, tea.Quit

	}

	// Count trial failures
	m.trialFailureCount = 0
	for _, node := range m.trials {
		v, err := node.Pipe(yaml.Lookup("status", "phase"))
		if err != nil {
			return m, func() tea.Msg { return err }
		}
		if strings.EqualFold(v.YNode().Value, "Failed") {
			m.trialFailureCount++
		}
	}
	if m.trialFailureCount > 10 {
		return m, tea.Quit
	}

	return m, nil
}