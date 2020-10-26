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

package experiment

import (
	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// applyApplicationMetadata updates the metadata of the generated objects.
func applyApplicationMetadata(app *Application, list *corev1.List) error {

	for i := range list.Items {
		switch obj := list.Items[i].Object.(type) {
		case *redskyv1beta1.Experiment:
			// TODO Do we want to filter out any of this information? Re-format it (e.g. "{appName}-{version}"?
			app.ObjectMeta.DeepCopyInto(&obj.ObjectMeta)
		}
	}

	return nil
}
