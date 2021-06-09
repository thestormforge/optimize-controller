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
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

// NOTE that even though completions are I/O, we want them to block so other
// events aren't processed until the completion.

type Completions interface {
	Suggest(value string) []string
	View(suggestion string) string
}

// SuggestionMsg should be used to update the value of the corresponding model.
type SuggestionMsg string

// CompletionModel is used to add completion functionality to another model via
// composition. If the completion function is set, it will enable tab completion
// via the update model (which must be given the current value of the model).
type CompletionModel struct {
	// Returns the possible values given the current value of the model.
	Completions

	suggestions     []string
	suggestionIndex int
}

// Update the model and return the result.
// NOTE: we do not return a `tea.Cmd` because this is expected to run
// synchronously to avoid unexpected delays in the user interaction.
func (m CompletionModel) Update(msg tea.Msg, value string) (CompletionModel, tea.Msg) {
	if m.Completions == nil {
		return m, msg
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyTab:
			if len(m.suggestions) > 0 {
				// The current value is a suggestion, assume we are cycling through
				m.selectSuggestions(msg)
			} else {
				// Generate a new set of suggestions
				m.resetSuggestions()
				suggestions := m.Completions.Suggest(value)
				if len(suggestions) == 1 {
					return m, SuggestionMsg(suggestions[0])
				} else if len(suggestions) > 1 {
					// If there are multiple choices but they have a common
					// prefix, just force the common part
					if prefix := longestCommonPrefix(suggestions); len(prefix) > len(value) {
						return m, SuggestionMsg(prefix)
					}
				}

				// Record the new suggestions
				m.suggestions = suggestions
			}

			// If the suggestion index is valid, suggest the value
			if m.suggestionIndex < len(m.suggestions) {
				return m, SuggestionMsg(m.suggestions[m.suggestionIndex])
			}

		case tea.KeyShiftTab, tea.KeyLeft, tea.KeyRight:
			if len(m.suggestions) > 0 {
				m.selectSuggestions(msg)
				if m.suggestionIndex < len(m.suggestions) {
					return m, SuggestionMsg(m.suggestions[m.suggestionIndex])
				}
			}

		default:
			// First key press resets the suggestions
			m.resetSuggestions()
		}
	}

	return m, msg
}

func (m *CompletionModel) resetSuggestions() {
	m.suggestionIndex = 0
	m.suggestions = nil
}

func (m *CompletionModel) selectSuggestions(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyTab, tea.KeyRight:
		m.suggestionIndex++
	case tea.KeyShiftTab, tea.KeyLeft:
		m.suggestionIndex--
	}

	if m.suggestionIndex >= len(m.suggestions) {
		m.suggestionIndex = 0
	}
	if m.suggestionIndex < 0 {
		m.suggestionIndex = len(m.suggestions)
	}
}

func (m CompletionModel) View() string {
	if m.Completions == nil || len(m.suggestions) <= 1 {
		return ""
	}

	// TODO Both View and selectSuggestions need to handle putting the options in columns

	var suggestions []string
	for i, s := range m.suggestions {
		ss := termenv.String(m.Completions.View(s))
		if i == m.suggestionIndex {
			ss = ss.Reverse()
		}
		suggestions = append(suggestions, ss.String())
	}

	return "\n" + strings.Join(suggestions, "   ")
}

func longestCommonPrefix(values []string) string {
	var min, max string
	switch len(values) {
	case 0:
		// No values, no prefix
		return ""
	case 1:
		// Single value _is_ the common prefix
		return values[0]
	case 2:
		// If there are only two values, order doesn't matter
		min, max = values[0], values[1]
	default:
		// The common prefix between the min and max holds for the whole set
		min, max := values[0], values[0]
		for _, value := range values[1:] {
			if value < min {
				min = value
			} else if value > max {
				max = value
			}
		}
	}

	// Prefix between two strings
	for i := 0; i < len(min) && i < len(max); i++ {
		if min[i] != max[i] {
			return min[:i]
		}
	}
	return min
}

// StaticCompletions is just a static list of allowed final values for a field.
type StaticCompletions []string

var _ Completions = StaticCompletions{}

// Suggest returns the values matching the supplied prefix.
func (s StaticCompletions) Suggest(prefix string) []string {
	var suggestions []string
	for _, suggestion := range s {
		if strings.HasPrefix(suggestion, prefix) {
			suggestions = append(suggestions, suggestion)
		}
	}
	return suggestions
}

// View returns the suggestion unmodified.
func (s StaticCompletions) View(suggestion string) string {
	return suggestion
}

type FileCompletions struct {
	// Working directory for relative completions.
	WorkingDirectory string
	// Flag indicating "." files can be shown.
	AllowHidden bool
	// Required extensions, empty means directories only, `[]string{"*"}` is all files.
	Extensions []string
}

var _ Completions = &FileCompletions{}

// Suggest returns directory contents matching the base name of the supplied path.
func (c *FileCompletions) Suggest(path string) []string {
	readDir, dir, file := c.split(path)
	entries, err := os.ReadDir(readDir)
	if err != nil {
		return nil
	}

	var suggestions []string
	for _, e := range entries {
		name := e.Name()

		if !strings.HasPrefix(name, file) {
			continue
		}

		if strings.HasPrefix(name, ".") && !c.AllowHidden {
			continue
		}

		if e.IsDir() {
			suggestions = append(suggestions, filepath.Join(dir, name)+"/")
		} else if c.hasExtension(filepath.Ext(name)) {
			suggestions = append(suggestions, filepath.Join(dir, name))
		}
	}

	return suggestions
}

// View returns the base name of the supplied suggestion.
func (c *FileCompletions) View(suggestion string) string {
	if strings.HasSuffix(suggestion, "/") {
		return filepath.Base(suggestion) + "/"
	}
	return filepath.Base(suggestion)
}

// ToggleHidden flips the allow hidden state.
func (c *FileCompletions) ToggleHidden() {
	c.AllowHidden = !c.AllowHidden
}

func (c *FileCompletions) hasExtension(ext string) bool {
	for _, e := range c.Extensions {
		if e == "*" || e == ext {
			return true
		}
	}
	return false
}

func (c *FileCompletions) split(path string) (readDir, dir, file string) {
	switch {
	case path == "~":
		readDir, _ = os.UserHomeDir()
		dir, file = path, ""
	case strings.HasPrefix(filepath.ToSlash(path), "~/"):
		home, _ := os.UserHomeDir()
		dir, file = filepath.Split(path)
		readDir = filepath.Clean(filepath.Join(home, dir[2:]))
	default:
		dir, file = filepath.Split(path)
		readDir = filepath.Clean(filepath.Join(c.WorkingDirectory, dir))
	}
	return
}
