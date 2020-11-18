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

package k8s

import (
	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// FindOrAddExperiment returns the experiment from the supplied list, creating it if it does not exist.
func FindOrAddExperiment(list *corev1.List) *redskyv1beta1.Experiment {
	var exp *redskyv1beta1.Experiment
	for i := range list.Items {
		if p, ok := list.Items[i].Object.(*redskyv1beta1.Experiment); ok {
			exp = p
			break
		}
	}

	if exp == nil {
		exp = &redskyv1beta1.Experiment{}
		list.Items = append(list.Items, runtime.RawExtension{Object: exp})
	}

	return exp
}

// EnsureSetupServiceAccount ensures that we are using an explicit service account for setup tasks.
func EnsureSetupServiceAccount(list *corev1.List) {
	// Return if we see an explicit service account name
	exp := FindOrAddExperiment(list)
	saName := &exp.Spec.TrialTemplate.Spec.SetupServiceAccountName
	if *saName != "" {
		return
	}
	*saName = "redsky-setup"

	// Add the actual service account to the list
	list.Items = append(list.Items,
		runtime.RawExtension{Object: &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: *saName,
			},
		}},
	)
}
