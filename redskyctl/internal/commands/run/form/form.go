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

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Start initiates the start of a form. Note that there is no implicit scoping
// of start messages to specific forms, proper routing of the start message is
// the responsibility of the enclosing model.
func Start() tea.Msg {
	return startMsg{}
}

type startMsg struct{}

// ValidationMsg is used to asynchronously validate form fields. An empty string
// is used to indicate a valid field while any non-empty strings indicates an
// error with the current field value. Validation messages are typically rendered
// in the view of a field and will be cleared on the first key press after the
// validation occurs.
type ValidationMsg string

// FinishedMsg indicates that the last field on the form has been submitted.
type FinishedMsg struct{}

// Fields is a list of fields that make up a form. When constructing a form, be
// sure to use pointers to the individual field structs at the time of the update.
// Most Bubble Tea updates operate on values, not references, so this is a little
// different.
type Fields []Field

// Init returns a command to invoke in response to a form started message.
func (f Fields) Init() tea.Cmd {
	// In theory we could check to see if these are needed but it's easier to
	// to fire off the single message and have it ignored.
	return tea.Batch(textinput.Blink, spinner.Tick)
}

// Update manages the transition when individual fields are submitted.
// IMPORTANT: the message cannot be delivered to the individual fields, they must
// still be updated (presumably after the form itself has had a chance to update).
func (f Fields) Update(msg tea.Msg) tea.Cmd {
	focused, next := f.activeFields()

	var cmds []tea.Cmd
	switch msg := msg.(type) {

	case startMsg:
		// Focus the next field if nothing is currently focused
		cmds = append(cmds, f.Init())
		if focused == nil && next != nil {
			next.Focus()
			next.Show()
		}

	case tea.KeyMsg:
		// Validate the focused field when the user hits "enter"
		if msg.Type == tea.KeyEnter && focused != nil {
			cmds = append(cmds, focused.Validate())
		}

	case ValidationMsg:
		// On successful validation, progress to the next field or finish the form
		if msg == "" {
			if focused != nil {
				focused.Blur()
			}

			if next != nil {
				next.Focus()
				next.Show()
			} else {
				cmds = append(cmds, func() tea.Msg { return FinishedMsg{} })
			}
		}

	}

	return tea.Batch(cmds...)
}

// View returns the views of all enabled, non-hidden fields (each with a trailing newline).
func (f Fields) View() string {
	var view strings.Builder
	for i := range f {
		if !f[i].Enabled() || f[i].Hidden() {
			continue
		}

		view.WriteString(f[i].View())
		view.WriteRune('\n')
	}
	return view.String()
}

// activeFields returns the current focused field and the next enabled field. If
// no field has focus, focused will be nil and next will be the first enabled
// field (if such a field exists).
func (f Fields) activeFields() (focused, next Field) {
	var first Field
	for i := range f {
		if !f[i].Enabled() {
			continue
		}

		if focused != nil {
			next = f[i]
			return
		}

		if f[i].Focused() {
			focused = f[i]
		}

		if first == nil {
			first = f[i]
		}
	}
	if focused == nil {
		return nil, first
	}
	return
}
