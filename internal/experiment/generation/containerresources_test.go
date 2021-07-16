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

package generation

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func TestContainerResourcesParameter(t *testing.T) {
	cases := []struct {
		desc string
		containerResourcesParameter
		expectedParameters []optimizev1beta2.Parameter
		expectedPatch      string
	}{
		{
			desc: "binary memory",

			containerResourcesParameter: containerResourcesParameter{
				pnode: pnode{
					fieldPath: []string{"spec", "resources"},
					value: encodeResourceRequirements(corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					}),
				},
				resources: []corev1.ResourceName{corev1.ResourceMemory},
			},

			expectedParameters: []optimizev1beta2.Parameter{
				{
					Name:     "memory",
					Baseline: newInt(2048),
					Min:      1024,
					Max:      4096,
				},
			},
			expectedPatch: unindent(`
              spec:
                resources:
                  limits:
                    memory: "{{ .Values.memory }}Mi"
                  requests:
                    memory: "{{ .Values.memory }}Mi"`),
		},

		{
			desc: "decimal memory",

			containerResourcesParameter: containerResourcesParameter{
				pnode: pnode{
					fieldPath: []string{"spec", "resources"},
					value: encodeResourceRequirements(corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("2G"),
						},
					}),
				},
				resources: []corev1.ResourceName{corev1.ResourceMemory},
			},

			expectedParameters: []optimizev1beta2.Parameter{
				{
					Name:     "memory",
					Baseline: newInt(2000),
					Min:      1000,
					Max:      4000,
				},
			},
			expectedPatch: unindent(`
              spec:
                resources:
                  limits:
                    memory: "{{ .Values.memory }}M"
                  requests:
                    memory: "{{ .Values.memory }}M"`),
		},

		{
			desc: "cpu",

			containerResourcesParameter: containerResourcesParameter{
				pnode: pnode{
					fieldPath: []string{"spec", "resources"},
					value: encodeResourceRequirements(corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2.0"),
						},
					}),
				},
				resources: []corev1.ResourceName{corev1.ResourceCPU},
			},

			expectedParameters: []optimizev1beta2.Parameter{
				{
					Name:     "cpu",
					Baseline: newInt(2000),
					Min:      1000,
					Max:      4000,
				},
			},
			expectedPatch: unindent(`
              spec:
                resources:
                  limits:
                    cpu: "{{ .Values.cpu }}m"
                  requests:
                    cpu: "{{ .Values.cpu }}m"`),
		},

		{
			desc: "tiny memory",

			containerResourcesParameter: containerResourcesParameter{
				pnode: pnode{
					fieldPath: []string{"spec", "resources"},
					value: encodeResourceRequirements(corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("256Ki"),
						},
					}),
				},
				resources: []corev1.ResourceName{corev1.ResourceMemory},
			},

			expectedParameters: []optimizev1beta2.Parameter{
				{
					Name:     "memory",
					Baseline: newInt(256),
					Min:      128,
					Max:      512,
				},
			},
			expectedPatch: unindent(`
              spec:
                resources:
                  limits:
                    memory: "{{ .Values.memory }}Ki"
                  requests:
                    memory: "{{ .Values.memory }}Ki"`),
		},

		{
			desc: "tiny cpu",

			containerResourcesParameter: containerResourcesParameter{
				pnode: pnode{
					fieldPath: []string{"spec", "resources"},
					value: encodeResourceRequirements(corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1.0"),
						},
					}),
				},
				resources: []corev1.ResourceName{corev1.ResourceCPU},
			},

			expectedParameters: []optimizev1beta2.Parameter{
				{
					Name:     "cpu",
					Baseline: newInt(1000),
					Min:      500,
					Max:      2000,
				},
			},
			expectedPatch: unindent(`
              spec:
                resources:
                  limits:
                    cpu: "{{ .Values.cpu }}m"
                  requests:
                    cpu: "{{ .Values.cpu }}m"`),
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			t.Run("parameters", func(t *testing.T) {
				parameters, err := c.containerResourcesParameter.Parameters(ignoreMetaForName)
				if assert.NoError(t, err) {
					assert.Equal(t, c.expectedParameters, parameters)
				}
			})

			t.Run("patch", func(t *testing.T) {
				filter, err := c.containerResourcesParameter.Patch(ignoreMetaForName)
				if assert.NoError(t, err) {
					patch, err := yaml.NewMapRNode(nil).Pipe(filter)
					if assert.NoError(t, err) {
						actual, err := yaml.String(patch.YNode())
						require.NoError(t, err)
						assert.YAMLEq(t, c.expectedPatch, actual)
					}
				}
			})
		})
	}
}

// encodeResourceRequirements is a helper to generate the YAML content necessary
// for the pnode value of the containerResourcesParameter.
func encodeResourceRequirements(rr corev1.ResourceRequirements) *yaml.Node {
	data, err := json.Marshal(rr)
	if err != nil {
		panic(err)
	}
	return yaml.MustParse(string(data)).YNode()
}

// newInt returns a pointer to a new IntOrString from the supplied int.
func newInt(val int) *intstr.IntOrString {
	result := intstr.FromInt(val)
	return &result
}

// ignoreMetaForName just returns the name, ignoring all other metadata.
func ignoreMetaForName(_ yaml.ResourceMeta, _ []string, name string) string { return name }

// unindent removes a fixed indentation width from each line of a string.
func unindent(s string) string {
	p := 0
	var result []string
	for _, line := range strings.Split(s, "\n") {
		if p == 0 {
			p = len(regexp.MustCompile(`^(\s+)`).FindString(line))
		}
		if len(line) > p {
			line = line[p:]
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}
