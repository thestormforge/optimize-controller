/*
Copyright 2020 GramLabs, Inc.

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

package experiments

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNames(t *testing.T) {
	cases := []struct {
		desc  string
		args  []string
		names []name
		err   string
	}{
		{
			desc:  "ListType",
			args:  []string{"exp"},
			names: []name{{Type: typeExperiment, Number: -1}},
		},
		{
			desc: "SharedType",
			args: []string{"experiment", "foo", "bar"},
			names: []name{
				{Type: typeExperiment, Name: "foo", Number: -1},
				{Type: typeExperiment, Name: "bar", Number: -1},
			},
		},
		{
			desc: "IndividualType",
			args: []string{"experiment/foo", "trial/bar"},
			names: []name{
				{Type: typeExperiment, Name: "foo", Number: -1},
				{Type: typeTrial, Name: "bar", Number: -1},
			},
		},
		{
			desc: "OverrideSharedType",
			args: []string{"experiment", "foo", "trial/bar"},
			names: []name{
				{Type: typeExperiment, Name: "foo", Number: -1},
				{Type: typeTrial, Name: "bar", Number: -1},
			},
		},
		{
			desc: "SharedTypeNumbered",
			args: []string{"trial", "foo/1", "foo/002", "foo-3", "foo-004"},
			names: []name{
				{Type: typeTrial, Name: "foo", Number: 1},
				{Type: typeTrial, Name: "foo", Number: 2},
				{Type: typeTrial, Name: "foo", Number: 3},
				{Type: typeTrial, Name: "foo", Number: 4},
			},
		},
		{
			desc: "TypeNumbered",
			args: []string{"trial/foo/1", "trial/foo/002", "trial/foo-3", "trial/foo-004"},
			names: []name{
				{Type: typeTrial, Name: "foo", Number: 1},
				{Type: typeTrial, Name: "foo", Number: 2},
				{Type: typeTrial, Name: "foo", Number: 3},
				{Type: typeTrial, Name: "foo", Number: 4},
			},
		},
		{
			desc: "Spaced",
			args: []string{"experiment", "Foo Bar"},
			names: []name{
				{Type: typeExperiment, Name: "Foo Bar", Number: -1},
			},
		},
		{
			desc:  "NotSpecified",
			args:  nil,
			names: nil,
			err:   "required resource not specified",
		},
		{
			desc:  "UnknownType",
			args:  []string{"foo"},
			names: nil,
			err:   "unknown resource type \"foo\"",
		},
		{
			desc: "ExperimentNumbered",
			args: []string{"experiment/foo-2"},
			names: []name{
				{Type: typeExperiment, Name: "foo-2", Number: -1},
			},
		},
		{
			desc: "LateType",
			args: []string{"experiment/foo", "trial", "foo-2"},
			names: []name{
				{Type: typeExperiment, Name: "foo", Number: -1},
				{Type: typeTrial, Name: "foo", Number: 2},
			},
		},
		{
			desc: "NameIsExperiment",
			args: []string{"experiment", "experiment", "experiment/experiment", "trial/experiment"},
			names: []name{
				{Type: typeExperiment, Name: "experiment", Number: -1},
				{Type: typeExperiment, Name: "experiment", Number: -1},
				{Type: typeTrial, Name: "experiment", Number: -1},
			},
		},
		{
			desc: "NameIsTrial",
			args: []string{"trial", "trial", "trial/trial", "experiment/trial"},
			names: []name{
				{Type: typeTrial, Name: "trial", Number: -1},
				{Type: typeTrial, Name: "trial", Number: -1},
				{Type: typeExperiment, Name: "trial", Number: -1},
			},
		},
		{
			desc: "ExperimentForListHasNumber",
			args: []string{"trials", "foo-2", "trials/foo-2"},
			names: []name{
				{Type: typeTrial, Name: "foo-2", Number: -1},
				{Type: typeTrial, Name: "foo-2", Number: -1},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			names, err := parseNames(c.args)
			if c.err != "" {
				assert.EqualError(t, err, c.err)
			} else if assert.NoError(t, err) {
				assert.Equal(t, c.names, names)
			}
		})
	}
}
