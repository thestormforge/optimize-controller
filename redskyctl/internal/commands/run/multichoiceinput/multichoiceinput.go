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
	"fmt"
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
	// Message to display while loading.
	LoadingMessage string
	// Instructional text to show after the choices.
	Instructions      string
	InstructionsColor string
	// Indicates the value does not need to match a choice.
	Editable bool
	// Don't give up focus if nothing is selected.
	Required bool

	highlighted  int
	selected     []int
	spinner      spinner.Model
	showRequired bool
}

func NewModel() Model {
	ti := textinput.NewModel()

	s := spinner.NewModel()
	s.Spinner = spinner.Line

	return Model{
		Model:             ti,
		InstructionsColor: "241",
		spinner:           s,
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.Focused() {
			switch msg.String() {
			case "up":
				m.SetHighlighted(m.highlighted - 1)

			case "down":
				m.SetHighlighted(m.highlighted + 1)

			case " ":
				m.Toggle(m.highlighted)

			}
		}
	case tea.WindowSizeMsg:
		m.SetHighlighted(m.highlighted) // Just a refresh
	}

	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	m.Model, cmd = m.Model.Update(msg)
	cmds = append(cmds, cmd)

	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) SetHighlighted(i int) {
	if len(m.Choices) == 0 {
		m.highlighted = 0
		return
	}

	switch {
	case i < 0:
		m.highlighted = len(m.Choices) - 1
	case i > len(m.Choices)-1:
		m.highlighted = 0
	default:
		m.highlighted = i
	}
}

func (m *Model) IsSelected(i int) bool {
	for _, s := range m.selected {
		if s == i {
			return true
		}
	}
	return false
}

func (m *Model) Values() []string {
	var values []string
	for _, s := range m.selected {
		values = append(values, m.Choices[s])
	}
	return values
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

func (m *Model) Toggle(i int) {
	if m.IsSelected(i) {
		m.Unselect(i)
	} else {
		m.Select(i)
	}
}

func (m *Model) TryBlur() bool {
	m.showRequired = false
	if m.Required && len(m.Values()) == 0 {
		m.showRequired = true
		return false
	}

	m.Blur()
	return true
}

func (m Model) View() string {
	var lines []string
	if m.Editable {
		lines = append(lines, m.Model.View())
	} else {
		lines = append(lines, m.Model.Prompt)
	}

	// TODO This needs to be in columns over 8 (might need right/left support)
	lines = append(lines, "")
	for i, c := range m.Choices {
		var line strings.Builder

		checkboxStyle := termenv.Style{}
		choiceStyle := termenv.Style{}
		if m.highlighted == i && m.Focused() {
			checkboxStyle = checkboxStyle.Bold()
			choiceStyle = choiceStyle.Bold()
		}

		line.WriteString(checkboxStyle.Styled("["))
		checked := " "
		for _, s := range m.selected {
			if s == i {
				checked = "x"
			}
		}
		line.WriteString(checked)
		line.WriteString(checkboxStyle.Styled("]"))
		line.WriteString(" ")
		line.WriteString(choiceStyle.Styled(c))

		lines = append(lines, line.String())
	}
	if len(m.Choices) == 0 {
		lines = append(lines, fmt.Sprintf("%s %s", m.spinner.View(), m.LoadingMessage))
	}
	lines = append(lines, "")

	if m.Focused() {
		instructionsStyle := termenv.Style{}.Foreground(termenv.ColorProfile().Color(m.InstructionsColor))

		instructions := instructionsStyle.Styled(m.Instructions)
		if m.showRequired {
			instructions += instructionsStyle.Styled("  |  ")
			requiredStyle := termenv.Style{}.Foreground(termenv.ANSIRed)
			instructions += requiredStyle.Styled("value is required")
		}

		lines = append(lines, instructions)
	}

	return strings.Join(lines, "\n")
}
