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

package validation

import (
	"fmt"

	redskyv1alpha1 "github.com/redskyops/redskyops-controller/pkg/apis/redsky/v1alpha1"
	redskyapi "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
)

// CheckDefinition will make sure the cluster and API experiment definitions are compatible
func CheckDefinition(exp *redskyv1alpha1.Experiment, ee *redskyapi.Experiment) error {
	if len(exp.Spec.Parameters) == len(ee.Parameters) {
		parameters := make(map[string]bool, len(exp.Spec.Parameters))
		for i := range exp.Spec.Parameters {
			parameters[exp.Spec.Parameters[i].Name] = true
		}
		for i := range ee.Parameters {
			delete(parameters, ee.Parameters[i].Name)
		}
		if len(parameters) > 0 {
			return fmt.Errorf("server and cluster have incompatible parameter definitions")
		}
	} else {
		return fmt.Errorf("server and cluster have incompatible parameter definitions")
	}

	if len(exp.Spec.Metrics) == len(ee.Metrics) {
		metrics := make(map[string]bool, len(exp.Spec.Metrics))
		for i := range exp.Spec.Metrics {
			metrics[exp.Spec.Metrics[i].Name] = true
		}
		for i := range ee.Metrics {
			delete(metrics, ee.Metrics[i].Name)
		}
		if len(metrics) > 0 {
			return fmt.Errorf("server and cluster have incompatible metric definitions")
		}
	} else {
		return fmt.Errorf("server and cluster have incompatible metric definitions")
	}

	return nil
}
