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
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type TextFieldValidator interface {
	ValidateTextField(value string) tea.Msg
}

type TextField struct {
	textinput.Model
	fieldModel
	CompletionModel
	Validator TextFieldValidator
}

var _ Field = &TextField{}

func NewTextField() TextField {
	return TextField{
		Model: textinput.NewModel(),
		fieldModel: fieldModel{
			Template: `{{ .Model.View }}{{ if .Error }}
{{ colorError .Error }}{{ else if .Focused }}{{ .CompletionModel.View }}{{ end }}{{ if .Focused }}
{{ colorInstructions .Instructions }}{{ end }}`,
			InstructionsColor: "241",
			ErrorColor:        "1",
			ErrorTextColor:    "1",
		},
		Validator: &unvalidated{},
	}
}

func (m TextField) Update(msg tea.Msg) (TextField, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	if m.Focused() {
		// CompletionModel.Update is synchronous, the message itself is updated
		m.CompletionModel, msg = m.CompletionModel.Update(msg, m.Model.Value())
		if msg, ok := msg.(SuggestionMsg); ok {
			m.Model.SetValue(string(msg))
			m.Model.CursorEnd()
		}

		m.fieldModel, cmd = m.fieldModel.update(msg)
		cmds = append(cmds, cmd)
	}

	m.Model, cmd = m.Model.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m TextField) View() string {
	// Since the receiver isn't a pointer, these changes only effect the
	// current view. This is desired because we do not want the change to
	// persist across calls.
	if m.Error != "" && m.ErrorTextColor != "" {
		m.TextColor = m.ErrorTextColor
	}
	return m.fieldModel.executeTemplate(&m)
}

func (m TextField) Validate() tea.Cmd {
	// Do not allow the field to be submitted with pending suggestions
	if len(m.suggestions) > 0 {
		return nil
	}

	value := m.Value()
	return func() tea.Msg { return m.Validator.ValidateTextField(value) }
}
