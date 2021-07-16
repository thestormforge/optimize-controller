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

package form

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/run/choiceinput"
)

type ChoiceFieldValidator interface {
	ValidateChoiceField(value string) tea.Msg
}

type ChoiceField struct {
	choiceinput.Model
	fieldModel
	Validator ChoiceFieldValidator
}

var _ Field = &ChoiceField{}

func NewChoiceField() ChoiceField {
	return ChoiceField{
		Model: choiceinput.NewModel(),
		fieldModel: fieldModel{
			Template: `{{ .Model.View }}{{ if .Error }}
{{ colorError .Error }}{{ end }}{{ if .Focused }}
{{ colorInstructions .Instructions }}{{ end }}`,
			InstructionsColor: "241",
			ErrorColor:        "1",
			ErrorTextColor:    "1",
		},
		Validator: &unvalidated{},
	}
}

func (m ChoiceField) Update(msg tea.Msg) (ChoiceField, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	if m.Focused() {
		m.fieldModel, cmd = m.fieldModel.update(msg)
		cmds = append(cmds, cmd)
	}

	m.Model, cmd = m.Model.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m ChoiceField) View() string {
	// Since the receiver isn't a pointer, these changes only effect the
	// current view. This is desired because we do not want the change to
	// persist across calls.
	if m.Error != "" && m.ErrorTextColor != "" {
		m.TextColor = m.ErrorTextColor
	}
	return m.fieldModel.executeTemplate(&m)
}

func (m ChoiceField) Validate() tea.Cmd {
	value := m.Value()
	return func() tea.Msg { return m.Validator.ValidateChoiceField(value) }
}
