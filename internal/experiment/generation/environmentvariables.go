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

package generation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/scan"
	"github.com/thestormforge/optimize-controller/v2/internal/sfio"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// EnvironmentVariablesSelector scans for environment variables.
type EnvironmentVariablesSelector struct {
	scan.GenericSelector
	// Regular expression matching the container name.
	ContainerName string `json:"containerName,omitempty"`
	// Name of the environment variable to match.
	VariableName string `json:"variableName,omitempty"`
	// Path to the environment variable's value.
	Path string `json:"path,omitempty"`
	// Prefix that appears before the value.
	ValuePrefix string `json:"valuePrefix,omitempty"`
	// Suffix that appears after the value.
	ValueSuffix string `json:"valueSuffix,omitempty"`
	// Allowed values for categorical parameters.
	Values []string `json:"values,omitempty"`
}

var _ scan.Selector = &EnvironmentVariablesSelector{}

func (s *EnvironmentVariablesSelector) Default() {
	if s.Kind == "" {
		s.Group = "apps|extensions"
		s.Kind = "Deployment|StatefulSet"
		s.Path = "/spec/template/spec/containers/[name={ .ContainerName }]/env/[name={ .VariableName }]/value"
	}
}

func (s *EnvironmentVariablesSelector) Map(node *yaml.RNode, meta yaml.ResourceMeta) ([]interface{}, error) {
	var result []interface{}

	path, err := sfio.FieldPath(s.Path, map[string]string{
		"ContainerName": s.ContainerName,
		"VariableName":  regexp.QuoteMeta(s.VariableName), // Quoting here ensures variable name is an exact match
	})
	if err != nil {
		return nil, err
	}

	return result, node.PipeE(sfio.TeeMatched(
		yaml.PathMatcher{Path: path},
		yaml.FilterFunc(func(node *yaml.RNode) (*yaml.RNode, error) {
			result = append(result, &environmentVariablesParameter{
				pnode: pnode{
					meta:      meta,
					fieldPath: node.FieldPath(),
					value:     node.YNode(),
				},
				prefix: s.ValuePrefix,
				suffix: s.ValueSuffix,
				values: s.Values,
			})
			return node, nil
		}),
	))
}

// environmentVariablesParameter is used to record the position of an environment variable specification
// found by the selector during scanning.
type environmentVariablesParameter struct {
	pnode
	prefix string
	suffix string
	values []string
}

var _ PatchSource = &environmentVariablesParameter{}
var _ ParameterSource = &environmentVariablesParameter{}

func (p *environmentVariablesParameter) Patch(name ParameterNamer) (yaml.Filter, error) {
	// Since the field path will contain a "[name=ENV_VAR]" we can just leave the name blank
	parameterName := name(p.meta, p.fieldPath, "")
	patch := fmt.Sprintf("%s{{ .Values.%s }}%s", p.prefix, parameterName, p.suffix)
	value := yaml.NewScalarRNode(patch)

	return yaml.Tee(
		&yaml.PathGetter{Path: p.fieldPath, Create: yaml.ScalarNode},
		yaml.FieldSetter{Value: value, OverrideStyle: true},
	), nil
}

func (p *environmentVariablesParameter) Parameters(name ParameterNamer) ([]optimizev1beta2.Parameter, error) {
	param := optimizev1beta2.Parameter{
		Name:     name(p.meta, p.fieldPath, ""),
		Baseline: new(intstr.IntOrString),
	}

	value := strings.TrimPrefix(strings.TrimSuffix(p.value.Value, p.suffix), p.prefix)
	if len(p.values) > 0 {
		if value == "" {
			value = p.values[0]
		}
		*param.Baseline = intstr.FromString(value)
		param.Values = appendMissing(p.values, value)
	} else if baseline, err := strconv.Atoi(value); err == nil {
		*param.Baseline = intstr.FromInt(baseline)
		param.Min = int32(baseline / 2)
		param.Max = int32(baseline * 2)
	} else {
		param.Baseline = nil
		param.Min = 100
		param.Max = 4000
	}

	return []optimizev1beta2.Parameter{param}, nil
}

func appendMissing(slice []string, elem string) []string {
	for _, s := range slice {
		if s == elem {
			return slice
		}
	}
	return append(slice, elem)
}
