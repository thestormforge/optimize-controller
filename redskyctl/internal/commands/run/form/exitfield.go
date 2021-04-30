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
)

type ExitField struct {
	Message string
	fieldModel
	focus bool
}

var _ Field = &ExitField{}

func NewExitField() ExitField {
	return ExitField{}
}

func (m ExitField) Update(msg tea.Msg) (ExitField, tea.Cmd) {
	if m.Focused() {
		return m, tea.Quit
	}

	return m, nil
}

func (m *ExitField) Focus() {
	m.focus = true
}

func (m ExitField) Focused() bool {
	return m.focus
}

func (m *ExitField) Blur() {
	m.focus = false
}

func (m ExitField) View() string {
	return m.Message
}

func (m ExitField) Validate() tea.Cmd {
	return nil
}
