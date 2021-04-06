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

package choiceinput

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
	Instructions string
	// Indicates the value does not need to match a choice.
	Editable bool

	selected int
	spinner  spinner.Model
}

func NewModel() Model {
	ti := textinput.NewModel()

	s := spinner.NewModel()
	s.Spinner = spinner.Line

	return Model{
		Model:   ti,
		spinner: s,
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.Focused() {
			switch msg.String() {
			case "up":
				m.SetSelected(m.selected - 1)

			case "down":
				m.SetSelected(m.selected + 1)

			}
		}
	case tea.WindowSizeMsg:
		m.SetSelected(m.selected) // Just a refresh
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

func (m *Model) SetSelected(i int) {
	if len(m.Choices) == 0 {
		m.selected = 0
		return
	}

	switch {
	case i < 0:
		m.selected = len(m.Choices) - 1
	case i > len(m.Choices)-1:
		m.selected = 0
	default:
		m.selected = i
	}

	m.Model.SetValue(m.Choices[m.selected])
	m.Model.CursorEnd()
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
		checked := " "
		if m.selected == i {
			checked = "x"
		}
		lines = append(lines, fmt.Sprintf("[%s] %s", checked, c))
	}
	if len(m.Choices) == 0 {
		lines = append(lines, fmt.Sprintf("%s %s", m.spinner.View(), m.LoadingMessage))
	}
	lines = append(lines, "")

	if m.Focused() {
		lines = append(lines, termenv.Style{}.Foreground(termenv.ColorProfile().Color("241")).Styled(m.Instructions))
	}

	return strings.Join(lines, "\n")
}
