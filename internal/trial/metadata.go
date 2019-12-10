/*
Copyright 2019 GramLabs, Inc.

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

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
)

// AddInitializer adds an initializer to the trial; returns true only if the trial is changed
func AddInitializer(t *redskyv1alpha1.Trial, initializer string) bool {
	annotation := strings.Split(t.GetAnnotations()[redskyv1alpha1.AnnotationInitializer], ",")
	for i := range annotation {
		annotation[i] = strings.TrimSpace(annotation[i])
		if annotation[i] == initializer {
			return false
		}
	}

	annotations := t.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[redskyv1alpha1.AnnotationInitializer] = strings.Join(append(annotation, initializer), ",")
	t.SetAnnotations(annotations)
	return true
}

// RemoveInitializer removes the first occurrence of an initializer from the trial; return true on if the trial is changed
func RemoveInitializer(t *redskyv1alpha1.Trial, initializer string) bool {
	annotation := strings.Split(t.GetAnnotations()[redskyv1alpha1.AnnotationInitializer], ",")
	for i := range initializer {
		annotation[i] = strings.TrimSpace(annotation[i])
		if annotation[i] == initializer {
			annotations := t.GetAnnotations()
			annotations[redskyv1alpha1.AnnotationInitializer] = strings.Join(append(annotation[:i], annotation[i+1:]...), ",")
			t.SetAnnotations(annotations)
			return true
		}
	}
	return false
}
