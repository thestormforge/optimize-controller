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
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

type testField struct {
	fieldModel
	focused bool
}

func (t *testField) Focus() {
	t.focused = true
}

func (t testField) Focused() bool {
	return t.focused
}

func (t *testField) Blur() {
	t.focused = false
}

func (t testField) View() string {
	return ""
}

func (t testField) Validate() tea.Cmd {
	return nil
}

func TestActiveFields(t *testing.T) {
	cases := []struct {
		desc    string
		fields  Fields
		focused int
		next    int
	}{
		{
			desc:    "empty",
			focused: -1,
			next:    -1,
		},
		{
			desc: "single enabled",
			fields: Fields{
				&testField{fieldModel: fieldModel{enabled: true}},
			},
			focused: -1, next: 0,
		},
		{
			desc: "single focused",
			fields: Fields{
				&testField{fieldModel: fieldModel{enabled: true}, focused: true},
			},
			focused: 0,
			next:    -1,
		},
		{
			desc: "all disabled",
			fields: Fields{
				&testField{},
				&testField{},
				&testField{},
			},
			focused: -1,
			next:    -1,
		},
		{
			desc: "skip disabled",
			fields: Fields{
				&testField{fieldModel: fieldModel{enabled: true}, focused: true},
				&testField{},
				&testField{fieldModel: fieldModel{enabled: true}},
			},
			focused: 0,
			next:    2,
		},
		{
			desc: "second field",
			fields: Fields{
				&testField{fieldModel: fieldModel{enabled: true}},
				&testField{fieldModel: fieldModel{enabled: true}, focused: true},
				&testField{fieldModel: fieldModel{enabled: true}},
			},
			focused: 1,
			next:    2,
		},
		{
			desc: "last focused",
			fields: Fields{
				&testField{fieldModel: fieldModel{enabled: true}},
				&testField{fieldModel: fieldModel{enabled: true}, focused: true},
			},
			focused: 1,
			next:    -1,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			actualFocused, actualNext := c.fields.activeFields()
			if c.focused < 0 {
				assert.Nil(t, actualFocused)
			} else {
				assert.Same(t, c.fields[c.focused], actualFocused)
			}
			if c.next < 0 {
				assert.Nil(t, actualNext)
			} else {
				assert.Same(t, c.fields[c.next], actualNext)
			}
		})
	}
}
