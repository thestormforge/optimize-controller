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

package util

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadExperiments(t *testing.T) {
	testCases := []struct {
		desc  string
		input []byte
	}{
		{
			desc:  "v1alpha1",
			input: v1alpha1experiment,
		},
		{
			desc:  "v1beta1",
			input: v1beta1experiment,
		},
		{
			desc:  "v1beta1s",
			input: v1beta1experiments,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			experiment, err := ReadExperiment("-", bytes.NewReader(tc.input))
			assert.NoError(t, err)
			assert.Equal(t, "postgres-example", experiment.Name)
			assert.Equal(t, int32(15), experiment.Spec.TrialTemplate.Spec.InitialDelaySeconds)

		})
	}
}

var v1beta1experiment = []byte(`apiVersion: redskyops.dev/v1beta1
kind: Experiment
metadata:
  name: postgres-example
spec:
  trialTemplate: # trial
    spec:
      initialDelaySeconds: 15
      jobtemplate: # job
        spec:
          template: # pod
            spec:
              containers:
              - image: crunchydata/crunchy-pgbench:centos7-11.4-2.4.1
                name: pgbench
                envFrom:
                - secretRef:
                    name: postgres-secret
  parameters:
  - name: memory
    min: 500
    max: 4000
  - name: cpu
    min: 100
    max: 4000`)

var v1alpha1experiment = []byte(`apiVersion: redskyops.dev/v1alpha1
kind: Experiment
metadata:
  name: postgres-example
spec:
  template: # trial
    spec:
      initialDelaySeconds: 15
      template: # job
        spec:
          template: # pod
            spec:
              containers:
              - image: crunchydata/crunchy-pgbench:centos7-11.4-2.4.1
                name: pgbench
                envFrom:
                - secretRef:
                    name: postgres-secret
  parameters:
  - name: memory
    min: 500
    max: 4000
  - name: cpu
    min: 100
    max: 4000`)

var v1beta1experiments = []byte(`---
apiVersion: redskyops.dev/v1beta1
kind: Experiment
metadata:
  name: postgres-example
spec:
  trialTemplate: # trial
    spec:
      initialDelaySeconds: 15
      jobtemplate: # job
        spec:
          template: # pod
            spec:
              containers:
              - image: crunchydata/crunchy-pgbench:centos7-11.4-2.4.1
                name: pgbench
                envFrom:
                - secretRef:
                    name: postgres-secret
  parameters:
  - name: memory
    min: 500
    max: 4000
  - name: cpu
    min: 100
    max: 4000
---
apiVersion: redskyops.dev/v1beta1
kind: Experiment
metadata:
  name: postgres-example2
spec:
  parameters:
  - name: memory
    min: 500
    max: 4000
  - name: cpu
    min: 100
    max: 4000`)
