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

package sfio

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func TestExperimentMigrationFilter_AppLabels(t *testing.T) {
	cases := []struct {
		desc            string
		applicationName string
		scenarioName    string
		experiment      *yaml.RNode
	}{
		{
			desc:            "simple name",
			applicationName: "postgres",
			scenarioName:    "default",
			experiment: yaml.MustParse(`
metadata:
  name: postgres-example
`),
		},
		{
			desc:            "common name",
			applicationName: "postgres",
			scenarioName:    "default",
			experiment: yaml.MustParse(`
metadata:
  name: postgres-example-final-final-3
`),
		},
		{
			desc:            "trailing int",
			applicationName: "myapp30",
			scenarioName:    "default",
			experiment: yaml.MustParse(`
metadata:
  name: myapp30
`),
		},
		{
			desc:            "isolated int",
			applicationName: "myapp",
			scenarioName:    "default",
			experiment: yaml.MustParse(`
metadata:
  name: myapp-40
`),
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			appName, scnName, err := (&ExperimentMigrationFilter{}).appLabels(c.experiment)
			if assert.NoError(t, err) {
				assert.Equal(t, c.applicationName, appName)
				assert.Equal(t, c.scenarioName, scnName)
			}
		})
	}
}
