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

package v1alpha1

import (
	"github.com/redskyops/redskyops-controller/api/v1beta1"
	conv "k8s.io/apimachinery/pkg/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func (in *Experiment) ConvertTo(hub conversion.Hub) error {
	s, err := SchemeBuilder.Build()
	if err != nil {
		return err
	}
	return s.Convert(in, hub.(*v1beta1.Experiment), nil)
}

func (in *Experiment) ConvertFrom(hub conversion.Hub) error {
	s, err := SchemeBuilder.Build()
	if err != nil {
		return err
	}
	return s.Convert(hub.(*v1beta1.Experiment), in, nil)
}

func Convert_v1alpha1_ExperimentSpec_To_v1beta1_ExperimentSpec(in *ExperimentSpec, out *v1beta1.ExperimentSpec, s conv.Scope) error {
	if err := Convert_v1alpha1_TrialTemplateSpec_To_v1beta1_TrialTemplateSpec(&in.Template, &out.TrialTemplate, s); err != nil {
		return err
	}
	return autoConvert_v1alpha1_ExperimentSpec_To_v1beta1_ExperimentSpec(in, out, s)
}

func Convert_v1beta1_ExperimentSpec_To_v1alpha1_ExperimentSpec(in *v1beta1.ExperimentSpec, out *ExperimentSpec, s conv.Scope) error {
	if err := Convert_v1beta1_TrialTemplateSpec_To_v1alpha1_TrialTemplateSpec(&in.TrialTemplate, &out.Template, s); err != nil {
		return err
	}
	return autoConvert_v1beta1_ExperimentSpec_To_v1alpha1_ExperimentSpec(in, out, s)
}
