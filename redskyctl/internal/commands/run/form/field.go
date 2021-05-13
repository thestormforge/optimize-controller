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

// Field represents an individual form field in a linear sequence of inputs.
// Initially disabled and hidden, fields should be enabled programmatically
// (either during initialization or in response to events related to the field);
// it is not necessary to show fields as they will be shown as the form progresses.
type Field interface {
	// Enable this field. Enabled fields can receive focus for updates and will be included in views.
	Enable()
	// Enabled returns true if this field currently enabled.
	Enabled() bool
	// Disable this field. Disabled fields will not be focused, will ignore most updates and will be excluded from views.
	Disable()

	// Focus this field so it can process update messages. Only one field per form should ever be focused.
	Focus()
	// Focused returns true if this field is processing updates.
	Focused() bool
	// Blur removes focus so this field stops processing update messages.
	Blur()

	// Show ensures an enabled field will be visible in a view.
	Show()
	// Hidden returns true if this field should not appear in a view.
	Hidden() bool
	// Hide ensures that this field will not be visible in a view.
	Hide()

	// View renders this field.
	View() string

	// Validate returns a command to check the current state of the field. The command MUST produce a `ValidationMsg`.
	Validate() tea.Cmd
}

// fieldModel tracks common field state.
type fieldModel struct {
	// A Go template to render an individual field. Ignored if the field is disabled or hidden.
	Template string

	// Instructional text to render with focused fields.
	Instructions string
	// Alternate color to be used when rendering instructions.
	InstructionsColor string

	// An error message related to validity of the field.
	Error string
	// Alternate color to be used when rendering error messages.
	ErrorColor string
	// For fields which display user input, an alternate text color to use for invalid input.
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

func (m *fieldModel) SetEnabled(enabled bool) {
	m.enabled = enabled
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

func (m *fieldModel) SetHidden(hidden bool) {
	m.shown = !hidden
}

func (m fieldModel) update(msg tea.Msg) (fieldModel, tea.Cmd) {
	if !m.Enabled() {
		return m, nil
	}

	switch msg := msg.(type) {
	case ValidationMsg:
		// Capture the error message from validation
		m.Error = string(msg)
	case tea.KeyMsg:
		// Clear the error message on the first key press
		m.Error = ""
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
