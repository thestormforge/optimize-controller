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
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// initializationModel is used at the beginning of the run command to print
// informational messages and initialize the controller.
// Note: the "version" strings are empty if no information is available (yet),
// once information is available they are either "unknown" or set to an actual
// version number.
type initializationModel struct {
	// Name of the tool, as invoked by the user.
	CommandName string
	// Version of the tool, as embedded by the build.
	BuildVersion string
	// Version of the forge command.
	ForgeVersion string
	// Version of the kubectl command.
	KubectlVersion string
	// Version of the currently running controller.
	ControllerVersion string
	// Image name of the controller to use for initialization.
	ControllerImage string
	// User authorization state as confirmed by the remote service.
	Authorization authorizationMsg
	// Percent of the initialization progress completed (0 to 1).
	InitializationPercent float64
}

// Update returns a copy of the model after applying the supplied message. If
// any further action is required, the returned command will be non-nil.
func (m initializationModel) Update(msg tea.Msg) (initializationModel, tea.Cmd) {
	switch msg := msg.(type) {

	case versionMsg:
		// Save non-empty version strings
		if v := msg.Build.Version; v != "" {
			m.BuildVersion = v
		}
		if v := msg.Forge; v != "" {
			m.ForgeVersion = v
		}
		if v := msg.Kubectl.ClientVersion.GitVersion; v != "" {
			m.KubectlVersion = v
		}
		if v := msg.Controller.Version; v != "" {
			m.ControllerVersion = v
		}

	case authorizationMsg:
		// Save changes to the authorization status
		m.Authorization = msg

	case tea.KeyMsg:
		// If the authorization is invalid, check to see if the user wants to ignore it
		if m.Authorization == azInvalid {
			switch msg.String() {
			case "y", "Y", "enter":
				return m, func() tea.Msg { return azIgnored }
			case "n", "N":
				return m, tea.Quit
			}
		}

	case initializedMsg:
		// Record a progress update (NOTE: this should only get set to 1 in the completed logic below)
		m.InitializationPercent = float64(msg)

	}

	// Check to see if the model completed
	if m.InitializationPercent < 1 &&
		m.BuildVersion != "" &&
		m.ForgeVersion != "" &&
		m.KubectlVersion != "" &&
		m.ControllerVersion != "" &&
		m.ControllerVersion != unknownVersion &&
		(m.Authorization == azValid || m.Authorization == azIgnored) {
		return m, func() tea.Msg { return initializedMsg(1) }
	}

	return m, nil
}

// generationModel is used to configure the experiment generator.
type generationModel struct {
	StormForgerTestCaseInput form.ChoiceField
	LocustfileInput          form.TextField

	NamespaceInput        form.MultiChoiceField
	LabelSelectorInputs   []form.TextField
	LabelSelectorTemplate func(namespace string) form.TextField

	IngressURLInput form.TextField

	ContainerResourcesSelectorInput form.TextField
	ReplicasSelectorInput           form.TextField

	ObjectiveInput form.MultiChoiceField
}

// form returns a slice of everything on generationModel that implements `form.Field`.
func (m *generationModel) form() form.Fields {
	var fields form.Fields
	fields = append(fields, &m.StormForgerTestCaseInput)
	fields = append(fields, &m.LocustfileInput)
	fields = append(fields, &m.NamespaceInput)
	for i := range m.LabelSelectorInputs {
		fields = append(fields, &m.LabelSelectorInputs[i])
	}
	fields = append(fields, &m.IngressURLInput)
	fields = append(fields, &m.ContainerResourcesSelectorInput, &m.ReplicasSelectorInput)
	fields = append(fields, &m.ObjectiveInput)
	return fields
}

func (m generationModel) Update(msg tea.Msg) (generationModel, tea.Cmd) {
	switch msg := msg.(type) {

	case initializedMsg:
		if msg >= 1 {
			return m, form.Start
		}

	case versionMsg:
		if msg.isForgeAvailable() {
			m.StormForgerTestCaseInput.Enable()
			m.LocustfileInput.Disable()
			m.IngressURLInput.Disable() // TODO For now this is exclusive to Locust
		}
		if msg.isKubectlAvailable() {
			m.NamespaceInput.Enable()
			for i := range m.LabelSelectorInputs {
				m.LabelSelectorInputs[i].Enable()
			}
		}

	case stormForgerTestCaseMsg:
		m.StormForgerTestCaseInput.Choices = append(m.StormForgerTestCaseInput.Choices, msg...)
		if len(m.StormForgerTestCaseInput.Choices) > 0 && m.StormForgerTestCaseInput.Value() == "" {
			m.StormForgerTestCaseInput.Select(0)
		}

	case kubernetesNamespaceMsg:
		m.NamespaceInput.Choices = append(m.NamespaceInput.Choices, msg...)
		if len(m.NamespaceInput.Choices) == 1 && len(m.NamespaceInput.Values()) == 0 {
			m.NamespaceInput.Select(0) // If there is only one namespace, select it
		}

	case form.FinishedMsg:
		return m, m.generateApplication

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			// If we just completed namespace selection, create the per-namespace label selector inputs
			if m.NamespaceInput.Focused() {
				namespaces := m.NamespaceInput.Values()
				m.LabelSelectorInputs = make([]form.TextField, len(namespaces))
				for i, namespace := range namespaces {
					m.LabelSelectorInputs[i] = m.LabelSelectorTemplate(namespace)
					m.LabelSelectorInputs[i].Enable()
				}
			}

		case tea.KeyCtrlCloseBracket:
			// Allow "tabbing" through the scenario types from the first input
			// TODO We should probably make a separate message for toggling test case type
			if m.StormForgerTestCaseInput.Enabled() && m.StormForgerTestCaseInput.Focused() {
				m.StormForgerTestCaseInput.Blur()
				m.StormForgerTestCaseInput.Disable()

				m.LocustfileInput.Focus()
				m.LocustfileInput.Show()
				m.LocustfileInput.Enable()
				m.IngressURLInput.Enable() // TODO For now this is exclusive to Locust
			} else if m.LocustfileInput.Enabled() && m.LocustfileInput.Focused() && len(m.StormForgerTestCaseInput.Choices) > 0 {
				m.StormForgerTestCaseInput.Focus()
				m.StormForgerTestCaseInput.Show()
				m.StormForgerTestCaseInput.Enable()

				m.LocustfileInput.Blur()
				m.LocustfileInput.Disable()
				m.IngressURLInput.Disable() // TODO For now this is exclusive to Locust
			}
		}

	}

	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	cmd = m.form().Update(msg)
	cmds = append(cmds, cmd)

	m.StormForgerTestCaseInput, cmd = m.StormForgerTestCaseInput.Update(msg)
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

type previewModel struct {
	experiment *redskyv1beta1.Experiment
	confirmed  bool
}

func (m previewModel) Update(msg tea.Msg) (previewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case experimentMsg:
		// Extract the experiment
		obj := scan.ObjectList{}
		if err := obj.Write(msg); err != nil {
			return m, func() tea.Msg { return err }
		}

		for i := range obj.Items {
			if exp, ok := obj.Items[i].Object.(*redskyv1beta1.Experiment); ok {
				m.experiment = exp
			}
		}

	case tea.KeyMsg:
		if m.experiment != nil && !m.confirmed {
			switch msg.String() {
			case "y", "Y", "enter":
				m.confirmed = true
				return m, func() tea.Msg { return expConfirmed }

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

	case experimentMsg:
		m.experiment = kio.ResourceNodeSlice(msg)

	case trialsMsg:
		m.trials = kio.ResourceNodeSlice(msg)

	case experimentStatusMsg:
		switch msg {
		case expCompleted:
			m.completed = true
			return m, tea.Quit
		case expFailed:
			m.failed = true
			return m, tea.Quit
		}

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlT:
			if len(m.experiment) > 0 {
				return m, func() tea.Msg {
					var buf strings.Builder
					p := kio.Pipeline{
						Inputs:  []kio.Reader{m.experiment},
						Filters: []kio.Filter{filters.FormatFilter{}},
						Outputs: []kio.Writer{&kio.ByteWriter{Writer: &buf}},
					}
					_ = p.Execute()
					return statusMsg(buf.String())
				}
			}
		}

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
