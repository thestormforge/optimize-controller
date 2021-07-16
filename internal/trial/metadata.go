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

package trial

import (
	"strings"

	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
)

// GetInitializers returns the initializers for the specified trial
func GetInitializers(t *optimizev1beta2.Trial) []string {
	var initializers []string
	for _, e := range strings.Split(t.GetAnnotations()[optimizev1beta2.AnnotationInitializer], ",") {
		e = strings.TrimSpace(e)
		if e != "" {
			initializers = append(initializers, e)
		}
	}
	return initializers
}

// SetInitializers sets the supplied initializers on the trial
func SetInitializers(t *optimizev1beta2.Trial, initializers []string) {
	a := t.GetAnnotations()
	if a == nil {
		a = make(map[string]string, 1)
	}
	if len(initializers) > 0 {
		a[optimizev1beta2.AnnotationInitializer] = strings.Join(initializers, ",")
	} else {
		delete(a, optimizev1beta2.AnnotationInitializer)
	}
	t.SetAnnotations(a)
}

// AddInitializer adds an initializer to the trial; returns true only if the trial is changed
func AddInitializer(t *optimizev1beta2.Trial, initializer string) bool {
	init := GetInitializers(t)
	for _, e := range init {
		if e == initializer {
			return false
		}
	}
	SetInitializers(t, append(init, initializer))
	return true
}

// RemoveInitializer removes the first occurrence of an initializer from the trial; returns true only if the trial is changed
func RemoveInitializer(t *optimizev1beta2.Trial, initializer string) bool {
	init := GetInitializers(t)
	for i, e := range init {
		if e == initializer {
			SetInitializers(t, append(init[:i], init[i+1:]...))
			return true
		}
	}
	return false
}
