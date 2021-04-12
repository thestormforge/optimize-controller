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
	"strings"
	"text/template"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

type Field interface {
	// Enable/disable: passes focus to next element, not shown, no updates

	Enable()
	Enabled() bool
	Disable()

	// Focus/blur: gets updates

	Focus()
	Focused() bool
	Blur()

	// Show/hide: get view

	Show()
	Hidden() bool
	Hide()

	// View

	View() string

	// Validate

	Validate() tea.Cmd
}

type fieldModel struct {
	Template string

	Instructions      string
	InstructionsColor string

	Error          string
	ErrorColor     string
	ErrorTextColor string

	enabled bool
	shown   bool
}

func (m fieldModel) Enabled() bool {
	return m.enabled
}

func (m *fieldModel) Enable() {
	m.enabled = true
}

func (m *fieldModel) Disable() {
	m.enabled = false
}

func (m fieldModel) Hidden() bool {
	return !m.shown
}

func (m *fieldModel) Show() {
	m.shown = true
}

func (m *fieldModel) Hide() {
	m.shown = false
}

func (m fieldModel) update(msg tea.Msg, focused bool) (fieldModel, tea.Cmd) {
	if !m.enabled {
		return m, nil
	}

	if focused {
		switch msg := msg.(type) {
		case ValidationMsg:
			m.Error = string(msg)
		case tea.KeyMsg:
			m.Error = ""
		}
	}

	return m, nil
}

func (m fieldModel) executeTemplate(data interface{}) string {
	// TODO Should we have a "DisabledMessage"?
	if !m.Enabled() || m.Hidden() {
		return ""
	}

	funcMap := template.FuncMap{
		"colorInstructions": colorFunc(m.InstructionsColor),
		"colorError":        colorFunc(m.ErrorColor),
	}

	t, err := template.New("field").Funcs(funcMap).Parse(m.Template)
	if err != nil {
		return err.Error()
	}

	var view strings.Builder
	if err := t.Execute(&view, data); err != nil {
		return err.Error()
	}

	return view.String()
}

func colorFunc(c string) func(s string) string {
	return termenv.Style{}.Foreground(termenv.ColorProfile().Color(c)).Styled
}
