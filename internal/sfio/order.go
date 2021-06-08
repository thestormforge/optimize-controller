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
	"reflect"
	"strings"

	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func init() {
	addFieldOrder(&optimizev1beta2.ExperimentSpec{}, 200)
	addFieldOrder(&optimizev1beta2.Parameter{}, 300)
	addFieldOrder(&optimizev1beta2.PatchTemplate{}, 400)
	addFieldOrder(&optimizev1beta2.Metric{}, 500)
	addFieldOrder(&optimizeappsv1alpha1.Application{}, 600)
}

// addFieldOrder use reflection to try and make the YAML sort order match the Go struct field order.
func addFieldOrder(obj interface{}, order int) {
	t := reflect.Indirect(reflect.ValueOf(obj)).Type()
	for i := 0; i < t.NumField(); i++ {
		if tag := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]; tag != "" {
			if _, ok := yaml.FieldOrder[tag]; !ok {
				yaml.FieldOrder[tag] = order
			}
			order++
		}
	}
}
