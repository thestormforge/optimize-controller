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

func (in *Trial) ConvertTo(hub conversion.Hub) error {
	s, err := SchemeBuilder.Build()
	if err != nil {
		return err
	}
	return s.Convert(in, hub.(*v1beta1.Trial), nil)
}

func (in *Trial) ConvertFrom(hub conversion.Hub) error {
	s, err := SchemeBuilder.Build()
	if err != nil {
		return err
	}
	return s.Convert(hub.(*v1beta1.Trial), in, nil)
}

func Convert_v1alpha1_Trial_To_v1beta1_Trial(in *Trial, out *v1beta1.Trial, s conv.Scope) error {
	if in.Spec.PatchOperations != nil {
		in, out := &in.Spec.PatchOperations, &out.Status.PatchOperations
		*out = make([]v1beta1.PatchOperation, len(*in))
		for i := range *in {
			if err := Convert_v1alpha1_PatchOperation_To_v1beta1_PatchOperation(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Status.PatchOperations = nil
	}

	if in.Spec.ReadinessChecks != nil {
		in, out := &in.Spec.ReadinessChecks, &out.Status.ReadinessChecks
		*out = make([]v1beta1.ReadinessCheck, len(*in))
		for i := range *in {
			if err := Convert_v1alpha1_ReadinessCheck_To_v1beta1_ReadinessCheck(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Status.ReadinessChecks = nil
	}

	return autoConvert_v1alpha1_Trial_To_v1beta1_Trial(in, out, s)
}

func Convert_v1beta1_Trial_To_v1alpha1_Trial(in *v1beta1.Trial, out *Trial, s conv.Scope) error {
	if in.Status.PatchOperations != nil {
		in, out := &in.Status.PatchOperations, &out.Spec.PatchOperations
		*out = make([]PatchOperation, len(*in))
		for i := range *in {
			if err := Convert_v1beta1_PatchOperation_To_v1alpha1_PatchOperation(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Spec.PatchOperations = nil
	}

	if in.Status.ReadinessChecks != nil {
		in, out := &in.Status.ReadinessChecks, &out.Spec.ReadinessChecks
		*out = make([]ReadinessCheck, len(*in))
		for i := range *in {
			if err := Convert_v1beta1_ReadinessCheck_To_v1alpha1_ReadinessCheck(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Spec.ReadinessChecks = nil
	}

	return autoConvert_v1beta1_Trial_To_v1alpha1_Trial(in, out, s)
}

func Convert_v1alpha1_TrialSpec_To_v1beta1_TrialSpec(in *TrialSpec, out *v1beta1.TrialSpec, s conv.Scope) error {
	out.JobTemplate = in.Template

	return autoConvert_v1alpha1_TrialSpec_To_v1beta1_TrialSpec(in, out, s)
}

func Convert_v1beta1_TrialSpec_To_v1alpha1_TrialSpec(in *v1beta1.TrialSpec, out *TrialSpec, s conv.Scope) error {
	out.Template = in.JobTemplate

	return autoConvert_v1beta1_TrialSpec_To_v1alpha1_TrialSpec(in, out, s)
}

func Convert_v1beta1_TrialStatus_To_v1alpha1_TrialStatus(in *v1beta1.TrialStatus, out *TrialStatus, s conv.Scope) error {
	return autoConvert_v1beta1_TrialStatus_To_v1alpha1_TrialStatus(in, out, s)
}
