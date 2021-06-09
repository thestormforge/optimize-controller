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

package pager

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type ExitMsg struct{}

type Model struct {
	viewport.Model
	// Return the instructions footer for a given width.
	Instructions func(width int) string

	// TODO `Silent bool` and add bell when we hit the top or bottom

	focus        bool
	instructions string
}

func NewModel() Model {
	return Model{}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.Model.Width = msg.Width
		m.Model.Height = msg.Height
		if m.Instructions != nil {
			m.instructions = m.Instructions(m.Model.Width)
			m.instructions = strings.Replace(m.instructions, "\r\n", "\n", -1)
			m.Model.Height -= strings.Count(m.instructions, "\n")
		}

	case ExitMsg:
		m.Blur()

	case tea.KeyMsg:
		if m.Focused() {
			switch msg.String() {
			case "q", "Q", "ctrl+x":
				cmds = append(cmds, func() tea.Msg { return ExitMsg{} })
			default:
				m.Model, cmd = m.Model.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	// There is no show/hide for the pager because it is full screen: either we
	// take over the whole screen and also accept key strokes, or we render
	// nothing and ignore key events.
	if !m.Focused() {
		return ""
	}
	return m.Model.View() + m.instructions
}

func (m *Model) Focus() {
	m.focus = true
}

func (m Model) Focused() bool {
	return m.focus
}

func (m *Model) Blur() {
	m.focus = false
}
