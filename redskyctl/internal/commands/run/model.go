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
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/scan"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/run/choiceinput"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/run/multichoiceinput"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// welcomeModel is used at the beginning of the run command to print informational messages.
type welcomeModel struct {
	CommandName           string
	BuildVersion          string
	ForgeVersion          string
	KubectlVersion        string
	ControllerVersion     string
	ControllerImage       string
	Authorization         authorizationMsg
	InitializationPercent float64
}

func (m welcomeModel) Update(msg tea.Msg) (welcomeModel, tea.Cmd) {
	switch msg := msg.(type) {

	case versionMsg:
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
		m.Authorization = msg

	case tea.KeyMsg:
		// If the authorization is invalid, check to see if the user wants to ignore it
		if m.Authorization == azInvalid {
			switch strings.ToLower(msg.String()) {
			case "y", "enter":
				return m, func() tea.Msg { return azIgnored }
			case "n":
				return m, tea.Quit
			}
		}

	}

	return m, nil
}

type scenarioMode int

const (
	scenarioStormForger scenarioMode = 1
	scenarioLocust      scenarioMode = 2
)

type resourceMode int

const (
	resourceNamespace resourceMode = 1
)

type generationModel struct {
	ScenarioMode    scenarioMode
	TestCaseInput   choiceinput.Model
	LocustNameInput textinput.Model
	LocustfileInput textinput.Model

	ResourceMode          resourceMode
	NamespaceInput        multichoiceinput.Model
	LabelSelectorTemplate textinput.Model // NOT USED, JUST CLONED
	LabelSelectorInputs   []textinput.Model

	IngressURLInput textinput.Model

	ContainerResourcesSelectorInput textinput.Model
	ReplicasSelectorInput           textinput.Model

	ObjectiveInput multichoiceinput.Model

	fieldIndex int // 0 = disabled, 1 = first field, ... , -1 = finished
	needsFocus bool
}

func (m generationModel) Update(msg tea.Msg) (generationModel, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {

	case versionMsg:
		installed := func(v string) bool { return v != "" && v != unknownVersion }

		if installed(msg.Kubectl.ClientVersion.GitVersion) {
			m.ResourceMode = resourceNamespace
		}
		if installed(msg.Forge) {
			m.ScenarioMode = scenarioStormForger
		}
		if installed(msg.Controller.Version) {
			m.fieldIndex = 1
			m.needsFocus = true
		}

	case stormForgerTestCaseMsg:
		m.TestCaseInput.Choices = append(m.TestCaseInput.Choices, msg...)

	case kubernetesNamespaceMsg:
		m.NamespaceInput.Choices = append(m.NamespaceInput.Choices, msg...)

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// If we just completed namespace selection, create the per-namespace label selector inputs
			if m.NamespaceInput.Focused() {
				for _, ns := range m.NamespaceInput.Values() {
					ls := m.LabelSelectorTemplate
					ls.Prompt = fmt.Sprintf(ls.Prompt, ns)
					m.LabelSelectorInputs = append(m.LabelSelectorInputs, ls)
				}
			}

			// Based on what is loosing focus, fire an event
			switch {
			case isLastFocused(m.scenarioFields()):
				cmds = append(cmds, m.generateScenarios)

			case isLastFocused(m.resourceFields()):
				cmds = append(cmds, m.generateResources)

			case isLastFocused(m.ingressFields()):
				cmds = append(cmds, m.generateIngress)

			case isLastFocused(m.parameterFields()):
				cmds = append(cmds, m.generateParameters)

			case isLastFocused(m.objectiveFields()):
				cmds = append(cmds, m.generateObjectives)
			}

			// Move the focus to the next field, if the result is negative we are done
			m.fieldIndex = focusNext(m.fields(), m.fieldIndex)
			if m.fieldIndex < 0 {
				cmds = append(cmds, func() tea.Msg { return expPreCreate })
			}

		case "tab":
			// Allow "tabbing" through the scenario types from the first input
			if sf := m.scenarioFields(); len(sf) > 0 && sf[0].Focused() {
				sf[0].Blur()
				m.needsFocus = true
				switch m.ScenarioMode {
				case scenarioStormForger:
					m.ScenarioMode = scenarioLocust
				case scenarioLocust:
					m.ScenarioMode = scenarioStormForger
				}
			}
		}

	}

	if m.needsFocus {
		if f := m.fields(); len(f) > 0 {
			f[0].Focus()
			m.needsFocus = false
		}
	}

	var cmd tea.Cmd

	m.TestCaseInput, cmd = m.TestCaseInput.Update(msg)
	cmds = append(cmds, cmd)

	m.LocustNameInput, cmd = m.LocustNameInput.Update(msg)
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

func (m *generationModel) fields() []input {
	ff := []func() []input{
		m.scenarioFields,
		m.resourceFields,
		m.ingressFields,
		m.parameterFields,
		m.objectiveFields,
	}

	// Collect up to the first nil set of fields. Stopping at the first nil
	// input slice ensures we can ask questions in the correct order if we are
	// waiting for async initialization of the input to complete.
	var result []input
	for _, f := range ff {
		if fs := f(); fs != nil {
			result = append(result, fs...)
			continue
		}
		return result
	}
	return result
}

func (m *generationModel) scenarioFields() []input {
	var result []input

	switch m.ScenarioMode {
	case scenarioStormForger:
		result = append(result, &m.TestCaseInput)
	case scenarioLocust:
		result = append(result, &m.LocustNameInput)
		result = append(result, &m.LocustfileInput)
	}

	return result
}

func (m *generationModel) resourceFields() []input {
	var result []input

	switch m.ResourceMode {
	case resourceNamespace:
		result = append(result, &m.NamespaceInput)
		for i := range m.LabelSelectorInputs {
			result = append(result, &m.LabelSelectorInputs[i])
		}
	}

	return result
}

func (m *generationModel) ingressFields() []input {
	result := make([]input, 0)

	// TODO Right now, only ask for URL with Locust
	if m.ScenarioMode == scenarioLocust {
		result = append(result, &m.IngressURLInput)
	}

	return result
}

func (m *generationModel) parameterFields() []input {
	result := make([]input, 0)

	result = append(result, &m.ContainerResourcesSelectorInput)
	result = append(result, &m.ReplicasSelectorInput)

	return result
}

func (m *generationModel) objectiveFields() []input {
	result := make([]input, 0)

	result = append(result, &m.ObjectiveInput)

	return result
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
			switch strings.ToLower(msg.String()) {
			case "y", "enter":
				m.confirmed = true
				return m, func() tea.Msg { return expConfirmed }

			case "n":
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
