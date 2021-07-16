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

package multichoiceinput

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

type Model struct {
	// Prompt and text input for the selection.
	textinput.Model

	// The list of possible choices.
	Choices []string

	// Indicates the value does not need to match a choice.
	Editable bool

	// Spinner to use while loading.
	LoadingSpinner spinner.Model
	// Message to display while loading.
	LoadingMessage string

	highlighted int
	selected    []int
}

func NewModel() Model {
	ti := textinput.NewModel()

	s := spinner.NewModel()
	s.Spinner = spinner.Line

	return Model{
		Model:          ti,
		LoadingSpinner: s,
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// Only update the text input if there are choices present
	if _, isKey := msg.(tea.KeyMsg); len(m.Choices) > 0 || !isKey {
		m.Model, cmd = m.Model.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.LoadingSpinner, cmd = m.LoadingSpinner.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.Focused() {
			switch msg.String() {
			case "up":
				m.Highlight(m.highlighted - 1)

			case "down":
				m.Highlight(m.highlighted + 1)

			case "x", " ":
				// TODO How do we handle this when we are editable?
				m.Toggle(m.highlighted)

			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) IsSelected(i int) bool {
	for _, s := range m.selected {
		if s == i {
			return true
		}
	}
	return false
}

func (m *Model) Select(i int) {
	for _, s := range m.selected {
		if s == i {
			return
		}
	}
	m.selected = append(m.selected, i)
	m.SetValue(strings.Join(m.Values(), ", "))
}

func (m *Model) Unselect(i int) {
	var selected []int
	for _, s := range m.selected {
		if s != i {
			selected = append(selected, s)
		}
	}
	m.selected = selected
	m.SetValue(strings.Join(m.Values(), ", "))
}

func (m *Model) SelectOnly() {
	if len(m.Choices) == 1 {
		m.Select(0)
	}
}

func (m *Model) Toggle(i int) {
	if m.IsSelected(i) {
		m.Unselect(i)
	} else {
		m.Select(i)
	}
}

func (m Model) IsHighlighted(i int) bool {
	return m.highlighted == i
}

func (m *Model) Highlight(i int) {
	switch {
	case i < 0:
		m.highlighted = len(m.Choices) - 1
	case i > len(m.Choices)-1:
		m.highlighted = 0
	default:
		m.highlighted = i
	}
}

func (m *Model) Values() []string {
	var values []string
	for _, s := range m.selected {
		values = append(values, m.Choices[s])
	}
	return values
}

func (m Model) View() string {
	var lines []string

	// Only render the whole text input if we allow edits
	if m.Editable && len(m.Choices) > 0 {
		lines = append(lines, m.Model.View())
	} else {
		lines = append(lines, m.Model.Prompt)
	}

	// If there are no choices yet, show the loading spinner/message
	if len(m.Choices) == 0 {
		lines = append(lines, "\n", m.LoadingSpinner.View(), m.LoadingMessage)
	}

	// Render the list of choices
	// TODO This needs to be in columns over 8 (might need right/left support)
	for i := range m.Choices {
		lines = append(lines, viewChoice(m.Choices[i], m.IsSelected(i), m.IsHighlighted(i), m.Focused()))
	}

	return strings.Join(lines, "")
}

func viewChoice(value string, selected, highlighted, focused bool) string {
	var choice strings.Builder
	var checkboxStyle, choiceStyle termenv.Style
	checked := " "

	if selected {
		checked = "x"
	}
	if highlighted && focused {
		checkboxStyle = checkboxStyle.Bold()
		choiceStyle = choiceStyle.Bold()
	}

	choice.WriteString("\n")
	choice.WriteString(checkboxStyle.Styled("["))
	choice.WriteString(checked)
	choice.WriteString(checkboxStyle.Styled("]"))
	choice.WriteString(" ")
	choice.WriteString(choiceStyle.Styled(value))

	return choice.String()
}
