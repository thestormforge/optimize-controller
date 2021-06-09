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

package out

import (
	"fmt"
	"strings"
	"text/tabwriter"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands/run/form"
)

type Style int

const (
	Happy Style = iota
	Sad
	ReallySad
	NotGood
	Version
	Authorized
	Unauthorized
	Initializing
	Ready
	Starting
	Running
	Watching
	Completed
	Failure
	Instructions
	YesNo
	Preview
)

var instructionsStyle = termenv.Style{}.Foreground(termenv.ColorProfile().Color("241"))

type statusOptions struct {
	Prefix      string
	OmitNewline bool
	termenv.Style
}

var statusConfig = map[Style]statusOptions{
	Happy:        {Prefix: "😄  "},
	Sad:          {Prefix: "😢  ", OmitNewline: true},
	ReallySad:    {Prefix: "😫  "},
	NotGood:      {Prefix: "😬  "},
	Version:      {Prefix: "    ▪ "},
	Authorized:   {Prefix: "🔑  "},
	Unauthorized: {Prefix: "🤷  "},
	Initializing: {Prefix: "💾  "},
	Ready:        {Prefix: "🎉  "},
	Starting:     {Prefix: "🚢  "},
	Running:      {Prefix: "👍  "},
	Watching:     {Prefix: "👓  "},
	Completed:    {Prefix: "🍾  "},
	Failure:      {Prefix: "❌  "},
	Instructions: {Style: instructionsStyle},
	YesNo:        {OmitNewline: true},
	Preview:      {},
}

// View is used to render a view line by line.
type View struct {
	lines []string
}

// Newline adds an empty line to this view.
func (v *View) Newline() {
	v.lines = append(v.lines, "\n")
}

// Step adds a stylized line to the view.
func (v *View) Step(style Style, format string, args ...interface{}) {
	s := statusConfig[style]
	v.lines = append(v.lines, s.Prefix+s.Styled(fmt.Sprintf(format, args...)))
	if !statusConfig[style].OmitNewline {
		v.lines = append(v.lines, "\n")
	}
}

// Model includes an entire sub-model in this view.
func (v *View) Model(m interface{ View() string }) {
	v.lines = append(v.lines, m.View())
}

// Write allows this view to be used as a writer.
func (v *View) Write(p []byte) (int, error) {
	v.lines = append(v.lines, string(p))
	return len(p), nil
}

// String returns the rendered view.
func (v *View) String() string {
	return strings.Join(v.lines, "")
}

// FieldOption can be used to apply common changes to a set of fields.
type FieldOption func(*FormField)

// VerbosePrompts is an option to enable verbose field prompts.
var VerbosePrompts FieldOption = func(field *FormField) {
	field.verbose = true
}

// GlobalInstructions appends the supplied instructions to every field.
func GlobalInstructions(ins ...interface{}) FieldOption {
	return func(field *FormField) {
		field.Instructions = append(field.Instructions, ins...)
	}
}

// KeyBinding represents a description of what a key should do.
type KeyBinding struct {
	// The key bound to the action.
	Key tea.Key
	// The description of the action.
	Desc string
}

// FormField is used to create new form fields with more consistency.
type FormField struct {
	Prompt          string
	PromptVerbose   string
	Placeholder     string
	LoadingMessage  string
	Instructions    []interface{}
	InputOnSameLine bool
	Choices         []string
	Completions     form.Completions

	verbose bool
}

// NewTextField creates a new text form field from the current state of this field spec.
func (f FormField) NewTextField(opts ...FieldOption) form.TextField {
	for _, opt := range opts {
		opt(&f)
	}

	field := form.NewTextField()
	field.Prompt = f.prompt()
	field.Placeholder = f.Placeholder
	field.Instructions = f.instructions()
	field.Completions = f.Completions
	if f.Completions == nil && len(f.Choices) > 0 {
		f.Completions = form.StaticCompletions(f.Choices)
	}

	return field
}

// NewChoiceField creates a new single choice form field from the current state of this field spec.
func (f FormField) NewChoiceField(opts ...FieldOption) form.ChoiceField {
	for _, opt := range opts {
		opt(&f)
	}

	field := form.NewChoiceField()
	field.Prompt = f.prompt()
	field.Placeholder = f.Placeholder
	field.Instructions = f.instructions()
	field.LoadingMessage = f.loadingMessage()
	field.Choices = f.Choices

	return field
}

// NewMultiChoiceField creates a new multiple choice form field from the current state of this field spec.
func (f FormField) NewMultiChoiceField(opts ...FieldOption) form.MultiChoiceField {
	for _, opt := range opts {
		opt(&f)
	}

	field := form.NewMultiChoiceField()
	field.Prompt = f.prompt()
	field.Placeholder = f.Placeholder
	field.Instructions = f.instructions()
	field.LoadingMessage = f.loadingMessage()
	field.Choices = f.Choices

	return field
}

// NewExitField creates a field that triggers an exit as soon as it is focused.
func (f FormField) NewExitField(opts ...FieldOption) form.ExitField {
	for _, opt := range opts {
		opt(&f)
	}

	field := form.NewExitField()
	field.Message = f.prompt()
	return field
}

func (f *FormField) prompt() string {
	var result strings.Builder
	result.WriteRune('\n')
	if f.verbose && f.PromptVerbose != "" {
		result.WriteString(f.PromptVerbose)
	} else {
		result.WriteString(f.Prompt)
	}
	if f.InputOnSameLine {
		result.WriteRune(' ')
	} else {
		result.WriteRune('\n')
	}
	return result.String()
}

func (f *FormField) instructions() string {
	if len(f.Instructions) == 0 {
		return ""
	}
	var text []string
	for _, ins := range f.Instructions {
		switch ins := ins.(type) {
		case string:
			text = append(text, ins)
		case KeyBinding:
			text = append(text, fmt.Sprintf("%s: %s", ins.Key, ins.Desc))
		}
	}
	return "\n" + strings.Join(text, "  |  ")
}

func (f *FormField) loadingMessage() string {
	if f.LoadingMessage == "" {
		return ""
	}
	return " " + f.LoadingMessage + " ..."
}

// PagerInstructions returns a nano-style instructions bar for the pager.
func PagerInstructions(keys []KeyBinding) func(width int) string {
	return func(width int) string {
		keyStyle := instructionsStyle.Reverse()
		descStyle := instructionsStyle

		var buf strings.Builder
		buf.WriteString("\n\n")
		tw := tabwriter.NewWriter(&buf, width/len(keys), 1, 1, ' ', 0)
		for _, k := range keys {
			_, _ = fmt.Fprintf(tw, "%s: %s\t", keyStyle.Styled(k.Key.String()), descStyle.Styled(k.Desc))
		}
		_ = tw.Flush()
		return buf.String()
	}
}
