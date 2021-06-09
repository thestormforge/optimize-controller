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
	"net/url"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type validator interface {
	TextFieldValidator
	ChoiceFieldValidator
	MultiChoiceFieldValidator
}

type unvalidated struct{}

func (u unvalidated) ValidateTextField(string) tea.Msg          { return ValidationMsg("") }
func (u unvalidated) ValidateChoiceField(string) tea.Msg        { return ValidationMsg("") }
func (u unvalidated) ValidateMultiChoiceField([]string) tea.Msg { return ValidationMsg("") }

type Required struct {
	Error       string
	IgnoreSpace bool
}

var _ validator = &Required{}

func (r *Required) ValidateTextField(value string) tea.Msg {
	if r.IgnoreSpace {
		value = strings.TrimSpace(value)
	}

	if value == "" {
		return ValidationMsg(r.Error)
	}

	return ValidationMsg("")
}

func (r *Required) ValidateChoiceField(value string) tea.Msg {
	if r.IgnoreSpace {
		value = strings.TrimSpace(value)
	}

	if value == "" {
		return ValidationMsg(r.Error)
	}

	return ValidationMsg("")
}

func (r *Required) ValidateMultiChoiceField(values []string) tea.Msg {
	if len(values) == 0 {
		return ValidationMsg(r.Error)
	}

	return ValidationMsg("")
}

type URL struct {
	Required   string
	InvalidURL string
	Absolute   string
}

func (v *URL) ValidateTextField(value string) tea.Msg {
	if v.Required != "" && value == "" {
		return ValidationMsg(v.Required)
	}
	u, err := url.Parse(value)
	if err != nil {
		return ValidationMsg(v.InvalidURL)
	}
	if v.Absolute != "" && !u.IsAbs() {
		return ValidationMsg(v.Absolute)
	}
	return ValidationMsg("")
}

type File struct {
	Required    string
	Missing     string
	Directory   string
	RegularFile string
}

func (v *File) ValidateTextField(value string) tea.Msg {
	if value == "" {
		return ValidationMsg(v.Required)
	}

	info, err := os.Lstat(value)
	if err != nil {
		if os.IsNotExist(err) {
			return ValidationMsg(v.Missing)
		}
		// TODO How should we handle this?
		return ValidationMsg(strings.TrimPrefix(err.Error(), "lstat "+value+": "))
	}

	if v.Directory != "" && !info.IsDir() {
		return ValidationMsg(v.Directory)
	}

	if v.RegularFile != "" && info.IsDir() {
		return ValidationMsg(v.RegularFile)
	}

	return ValidationMsg("")
}
