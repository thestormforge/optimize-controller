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
	"github.com/redskyops/redskyops-controller/internal/controller"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func (src *Assignment) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.Assignment)
	dst.Name = src.Name
	dst.Value = src.Value
	return nil
}

func (dst *Assignment) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.Assignment)
	dst.Name = src.Name
	dst.Value = src.Value
	return nil
}

func (src *TrialReadinessGate) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.TrialReadinessGate)
	dst.Kind = src.Kind
	dst.Name = src.Name
	dst.APIVersion = src.APIVersion
	dst.Selector = src.Selector
	if src.ConditionTypes != nil {
		in, out := &src.ConditionTypes, &dst.ConditionTypes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	dst.InitialDelaySeconds = src.InitialDelaySeconds
	dst.PeriodSeconds = src.PeriodSeconds
	dst.FailureThreshold = src.FailureThreshold
	return nil
}

func (dst *TrialReadinessGate) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.TrialReadinessGate)
	dst.Kind = src.Kind
	dst.Name = src.Name
	dst.APIVersion = src.APIVersion
	dst.Selector = src.Selector
	if src.ConditionTypes != nil {
		in, out := &src.ConditionTypes, &dst.ConditionTypes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	dst.InitialDelaySeconds = src.InitialDelaySeconds
	dst.PeriodSeconds = src.PeriodSeconds
	dst.FailureThreshold = src.FailureThreshold
	return nil
}

func (src *HelmValue) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.HelmValue)
	dst.Name = src.Name
	dst.ForceString = src.ForceString
	dst.Value = src.Value
	if src.ValueFrom != nil {
		in, out := &src.ValueFrom, &dst.ValueFrom
		*out = new(v1beta1.HelmValueSource)
		if err := (*in).ConvertTo(*out); err != nil {
			return err
		}
	}
	return nil
}

func (dst *HelmValue) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.HelmValue)
	dst.Name = src.Name
	dst.ForceString = src.ForceString
	dst.Value = src.Value
	if src.ValueFrom != nil {
		in, out := &src.ValueFrom, &dst.ValueFrom
		*out = new(HelmValueSource)
		if err := (*out).ConvertFrom(*in); err != nil {
			return err
		}
	}
	return nil
}

func (src *HelmValueSource) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.HelmValueSource)
	if src.ParameterRef != nil {
		in, out := &src.ParameterRef, &dst.ParameterRef
		*out = new(v1beta1.ParameterSelector)
		if err := (*in).ConvertTo(*out); err != nil {
			return err
		}
	}
	return nil
}

func (dst *HelmValueSource) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.HelmValueSource)
	if src.ParameterRef != nil {
		in, out := &src.ParameterRef, &dst.ParameterRef
		*out = new(ParameterSelector)
		if err := (*out).ConvertFrom(*in); err != nil {
			return err
		}
	}
	return nil
}

func (src *ParameterSelector) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.ParameterSelector)
	dst.Name = src.Name
	return nil
}

func (dst *ParameterSelector) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.ParameterSelector)
	dst.Name = src.Name
	return nil
}

func (src *HelmValuesFromSource) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.HelmValuesFromSource)
	if src.ConfigMap != nil {
		in, out := &src.ConfigMap, &dst.ConfigMap
		*out = new(v1beta1.ConfigMapHelmValuesFromSource)
		if err := (*in).ConvertTo(*out); err != nil {
			return err
		}
	}
	return nil
}

func (dst *HelmValuesFromSource) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.HelmValuesFromSource)
	if src.ConfigMap != nil {
		in, out := &src.ConfigMap, &dst.ConfigMap
		*out = new(ConfigMapHelmValuesFromSource)
		if err := (*out).ConvertFrom(*in); err != nil {
			return err
		}
	}
	return nil
}
func (src *ConfigMapHelmValuesFromSource) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.ConfigMapHelmValuesFromSource)
	dst.LocalObjectReference = src.LocalObjectReference
	return nil
}

func (dst *ConfigMapHelmValuesFromSource) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.ConfigMapHelmValuesFromSource)
	dst.LocalObjectReference = src.LocalObjectReference
	return nil
}

func (src *SetupTask) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.SetupTask)
	dst.Name = src.Name
	dst.Image = src.Image
	dst.SkipCreate = src.SkipCreate
	dst.SkipDelete = src.SkipDelete
	if src.VolumeMounts != nil {
		in, out := &src.VolumeMounts, &dst.VolumeMounts
		*out = make([]corev1.VolumeMount, len(*in))
		copy(*out, *in)
	}
	dst.HelmChart = src.HelmChart
	if src.HelmValues != nil {
		in, out := &src.HelmValues, &dst.HelmValues
		*out = make([]v1beta1.HelmValue, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	if src.HelmValuesFrom != nil {
		in, out := &src.HelmValuesFrom, &dst.HelmValuesFrom
		*out = make([]v1beta1.HelmValuesFromSource, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (dst *SetupTask) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.SetupTask)
	dst.Name = src.Name
	dst.Image = src.Image
	dst.SkipCreate = src.SkipCreate
	dst.SkipDelete = src.SkipDelete
	if src.VolumeMounts != nil {
		in, out := &src.VolumeMounts, &dst.VolumeMounts
		*out = make([]corev1.VolumeMount, len(*in))
		copy(*out, *in)
	}
	dst.HelmChart = src.HelmChart
	if src.HelmValues != nil {
		in, out := &src.HelmValues, &dst.HelmValues
		*out = make([]HelmValue, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	if src.HelmValuesFrom != nil {
		in, out := &src.HelmValuesFrom, &dst.HelmValuesFrom
		*out = make([]HelmValuesFromSource, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (src *PatchOperation) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.PatchOperation)
	dst.TargetRef = src.TargetRef
	dst.PatchType = src.PatchType
	if src.Data != nil {
		in, out := &src.Data, &dst.Data
		*out = make([]byte, len(*in))
		copy(*out, *in)
	}
	dst.AttemptsRemaining = src.AttemptsRemaining
	return nil
}

func (dst *PatchOperation) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.PatchOperation)
	dst.TargetRef = src.TargetRef
	dst.PatchType = src.PatchType
	if src.Data != nil {
		in, out := &src.Data, &dst.Data
		*out = make([]byte, len(*in))
		copy(*out, *in)
	}
	dst.AttemptsRemaining = src.AttemptsRemaining
	return nil
}

func (src *ReadinessCheck) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.ReadinessCheck)
	dst.TargetRef = src.TargetRef
	dst.Selector = src.Selector
	if src.ConditionTypes != nil {
		in, out := &src.ConditionTypes, &dst.ConditionTypes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	dst.InitialDelaySeconds = src.InitialDelaySeconds
	dst.PeriodSeconds = src.PeriodSeconds
	dst.AttemptsRemaining = src.AttemptsRemaining
	dst.LastCheckTime = src.LastCheckTime
	return nil
}

func (dst *ReadinessCheck) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.ReadinessCheck)
	dst.TargetRef = src.TargetRef
	dst.Selector = src.Selector
	if src.ConditionTypes != nil {
		in, out := &src.ConditionTypes, &dst.ConditionTypes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	dst.InitialDelaySeconds = src.InitialDelaySeconds
	dst.PeriodSeconds = src.PeriodSeconds
	dst.AttemptsRemaining = src.AttemptsRemaining
	dst.LastCheckTime = src.LastCheckTime
	return nil
}

func (src *Value) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.Value)
	dst.Name = src.Name
	dst.Value = src.Value
	dst.Error = src.Error
	dst.AttemptsRemaining = src.AttemptsRemaining
	return nil
}

func (dst *Value) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.Value)
	dst.Name = src.Name
	dst.Value = src.Value
	dst.Error = src.Error
	dst.AttemptsRemaining = src.AttemptsRemaining
	return nil
}

func (src *TrialCondition) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.TrialCondition)
	dst.Type = v1beta1.TrialConditionType(src.Type)
	dst.Status = src.Status
	dst.LastProbeTime = src.LastProbeTime
	dst.LastTransitionTime = src.LastTransitionTime
	dst.Reason = src.Reason
	dst.Message = src.Message
	return nil
}

func (dst *TrialCondition) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.TrialCondition)
	dst.Type = TrialConditionType(src.Type)
	dst.Status = src.Status
	dst.LastProbeTime = src.LastProbeTime
	dst.LastTransitionTime = src.LastTransitionTime
	dst.Reason = src.Reason
	dst.Message = src.Message
	return nil
}

// Do not expose TrialSpec/Status conversion individually except for internal use (i.e. the experiment's trial template)

func (src *TrialSpec) convertTo(dst *v1beta1.TrialSpec) error {
	dst.ExperimentRef = src.ExperimentRef
	if src.Assignments != nil {
		in, out := &src.Assignments, &dst.Assignments
		*out = make([]v1beta1.Assignment, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	dst.Selector = src.Selector
	dst.Template = src.Template
	dst.InitialDelaySeconds = src.InitialDelaySeconds
	dst.StartTimeOffset = src.StartTimeOffset
	dst.ApproximateRuntime = src.ApproximateRuntime
	dst.TTLSecondsAfterFinished = src.TTLSecondsAfterFinished
	dst.TTLSecondsAfterFailure = src.TTLSecondsAfterFailure
	if src.ReadinessGates != nil {
		in, out := &src.ReadinessGates, &dst.ReadinessGates
		*out = make([]v1beta1.TrialReadinessGate, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	if src.PatchOperations != nil {
		in, out := &src.PatchOperations, &dst.PatchOperations
		*out = make([]v1beta1.PatchOperation, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	if src.ReadinessChecks != nil {
		in, out := &src.ReadinessChecks, &dst.ReadinessChecks
		*out = make([]v1beta1.ReadinessCheck, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	if src.Values != nil {
		in, out := &src.Values, &dst.Values
		*out = make([]v1beta1.Value, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	if src.SetupTasks != nil {
		in, out := &src.SetupTasks, &dst.SetupTasks
		*out = make([]v1beta1.SetupTask, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	if src.SetupVolumes != nil {
		in, out := &src.SetupVolumes, &dst.SetupVolumes
		*out = make([]corev1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	dst.SetupServiceAccountName = src.SetupServiceAccountName
	if src.SetupDefaultRules != nil {
		in, out := &src.SetupDefaultRules, &dst.SetupDefaultRules
		*out = make([]rbacv1.PolicyRule, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return nil
}

func (dst *TrialSpec) convertFrom(src *v1beta1.TrialSpec) error {
	dst.ExperimentRef = src.ExperimentRef
	if src.Assignments != nil {
		in, out := &src.Assignments, &dst.Assignments
		*out = make([]Assignment, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	dst.Selector = src.Selector
	dst.Template = src.Template
	dst.InitialDelaySeconds = src.InitialDelaySeconds
	dst.StartTimeOffset = src.StartTimeOffset
	dst.ApproximateRuntime = src.ApproximateRuntime
	dst.TTLSecondsAfterFinished = src.TTLSecondsAfterFinished
	dst.TTLSecondsAfterFailure = src.TTLSecondsAfterFailure
	if src.ReadinessGates != nil {
		in, out := &src.ReadinessGates, &dst.ReadinessGates
		*out = make([]TrialReadinessGate, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	if src.PatchOperations != nil {
		in, out := &src.PatchOperations, &dst.PatchOperations
		*out = make([]PatchOperation, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	if src.ReadinessChecks != nil {
		in, out := &src.ReadinessChecks, &dst.ReadinessChecks
		*out = make([]ReadinessCheck, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	if src.Values != nil {
		in, out := &src.Values, &dst.Values
		*out = make([]Value, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	if src.SetupTasks != nil {
		in, out := &src.SetupTasks, &dst.SetupTasks
		*out = make([]SetupTask, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	if src.SetupVolumes != nil {
		in, out := &src.SetupVolumes, &dst.SetupVolumes
		*out = make([]corev1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	dst.SetupServiceAccountName = src.SetupServiceAccountName
	if src.SetupDefaultRules != nil {
		in, out := &src.SetupDefaultRules, &dst.SetupDefaultRules
		*out = make([]rbacv1.PolicyRule, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return nil
}

func (src *Trial) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.Trial)
	dst.ObjectMeta = src.ObjectMeta
	if err := src.Spec.convertTo(&dst.Spec); err != nil {
		return err
	}
	dst.Status.Phase = src.Status.Phase
	dst.Status.Assignments = src.Status.Assignments
	dst.Status.Values = src.Status.Values
	dst.Status.StartTime = src.Status.StartTime
	dst.Status.CompletionTime = src.Status.CompletionTime
	if src.Status.Conditions != nil {
		in, out := &src.Status.Conditions, &dst.Status.Conditions
		*out = make([]v1beta1.TrialCondition, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (dst *Trial) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.Trial)
	dst.ObjectMeta = src.ObjectMeta
	if err := dst.Spec.convertFrom(&src.Spec); err != nil {
		return err
	}
	dst.Status.Phase = src.Status.Phase
	dst.Status.Assignments = src.Status.Assignments
	dst.Status.Values = src.Status.Values
	dst.Status.StartTime = src.Status.StartTime
	dst.Status.CompletionTime = src.Status.CompletionTime
	if src.Status.Conditions != nil {
		in, out := &src.Status.Conditions, &dst.Status.Conditions
		*out = make([]TrialCondition, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	return nil
}
