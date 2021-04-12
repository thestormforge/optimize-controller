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

	tea "github.com/charmbracelet/bubbletea"
)

func Start() tea.Msg {
	return startMsg{}
}

type startMsg struct{}

type ValidationMsg string

type FinishedMsg struct{}

type Fields []Field

// DOES NOT attempt to deliver the message to the individual fields, independently updates them
func (f Fields) Update(msg tea.Msg) tea.Cmd {
	// Watch for an attempt to start the form, we need to show and focus the first enabled field
	if _, ok := msg.(startMsg); ok {
		startIndex := -1
		for i := range f {
			if !f[i].Enabled() {
				continue
			}
			if f[i].Focused() {
				startIndex = -1
				break
			}
			if startIndex < 0 {
				startIndex = i
			}
		}
		if startIndex >= 0 {
			f[startIndex].Focus()
		}
	}

	focused := false
	for i := range f {
		if !f[i].Focused() {
			continue
		}

		if !f[i].Enabled() || focused {
			f[i].Blur()
			continue
		}

		focused = true

		if f[i].Hidden() {
			f[i].Show()
		}

		switch msg := msg.(type) {

		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEnter:
				return f[i].Validate()
			}

		case ValidationMsg:
			if msg != "" {
				return nil
			}

			f[i].Blur()

			for n := i + 1; n < len(f); n++ {
				if !f[n].Enabled() {
					continue
				}

				f[n].Show()
				f[n].Focus()
				return nil
			}

			return func() tea.Msg { return FinishedMsg{} }
		}
	}

	return nil
}

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
