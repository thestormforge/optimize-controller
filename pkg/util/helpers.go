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
package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddFinalizer adds a finalizer to an object; returns true only if object is changed
func AddFinalizer(obj metav1.Object, finalizer string) bool {
	// Do not add the finalizer if the object is already deleted
	if !obj.GetDeletionTimestamp().IsZero() {
		return false
	}

	// Do not add the finalizer more then once
	finalizers := obj.GetFinalizers()
	for _, f := range finalizers {
		if f == finalizer {
			return false
		}
	}

	// Actually add the finalizer
	obj.SetFinalizers(append(finalizers, finalizer))
	return true
}

// RemoveFinalizer deletes a finalizer from an object; return true only if the object is changed
func RemoveFinalizer(obj metav1.Object, finalizer string) bool {
	finalizers := obj.GetFinalizers()
	for i := range finalizers {
		if finalizers[i] == finalizer {
			finalizers[i] = finalizers[len(finalizers)-1]
			obj.SetFinalizers(finalizers[:len(finalizers)-1])
			return true
		}
	}
	return false
}
