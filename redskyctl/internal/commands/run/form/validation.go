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

type validator interface {
	TextFieldValidator
	ChoiceFieldValidator
	MultiChoiceFieldValidator
}

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
