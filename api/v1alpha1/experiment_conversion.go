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
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func (src *Optimization) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.Optimization)
	dst.Name = src.Name
	dst.Value = src.Value
	return nil
}

func (dst *Optimization) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.Optimization)
	dst.Name = src.Name
	dst.Value = src.Value
	return nil
}

func (src *Parameter) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.Parameter)
	dst.Name = src.Name
	dst.Min = src.Min
	dst.Max = src.Max
	return nil
}

func (dst *Parameter) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.Parameter)
	dst.Name = src.Name
	dst.Min = src.Min
	dst.Max = src.Max
	return nil
}

func (src *Constraint) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.Constraint)
	dst.Name = src.Name
	if src.Order != nil {
		in, out := &src.Order, &dst.Order
		*out = new(v1beta1.OrderConstraint)
		if err := (*in).ConvertTo(*out); err != nil {
			return err
		}
	}
	if src.Sum != nil {
		in, out := &src.Sum, &dst.Sum
		*out = new(v1beta1.SumConstraint)
		if err := (*in).ConvertTo(*out); err != nil {
			return err
		}
	}
	return nil
}

func (dst *Constraint) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.Constraint)
	dst.Name = src.Name
	if src.Order != nil {
		in, out := &src.Order, &dst.Order
		*out = new(OrderConstraint)
		if err := (*out).ConvertFrom(*in); err != nil {
			return err
		}
	}
	if src.Sum != nil {
		in, out := &src.Sum, &dst.Sum
		*out = new(SumConstraint)
		if err := (*out).ConvertFrom(*in); err != nil {
			return err
		}
	}
	return nil
}

func (src *OrderConstraint) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.OrderConstraint)
	dst.LowerParameter = src.LowerParameter
	dst.UpperParameter = src.UpperParameter
	return nil
}

func (dst *OrderConstraint) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.OrderConstraint)
	dst.LowerParameter = src.LowerParameter
	dst.UpperParameter = src.UpperParameter
	return nil
}
func (src *SumConstraintParameter) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.SumConstraintParameter)
	dst.Name = src.Name
	dst.Weight = src.Weight
	return nil
}

func (dst *SumConstraintParameter) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.SumConstraintParameter)
	dst.Name = src.Name
	dst.Weight = src.Weight
	return nil
}
func (src *SumConstraint) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.SumConstraint)
	dst.Bound = src.Bound
	dst.IsUpperBound = src.IsUpperBound
	if src.Parameters != nil {
		in, out := &src.Parameters, &dst.Parameters
		*out = make([]v1beta1.SumConstraintParameter, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	return nil
}
func (dst *SumConstraint) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.SumConstraint)
	dst.Bound = src.Bound
	dst.IsUpperBound = src.IsUpperBound
	if src.Parameters != nil {
		in, out := &src.Parameters, &dst.Parameters
		*out = make([]SumConstraintParameter, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (src *Metric) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.Metric)
	dst.Name = src.Name
	dst.Minimize = src.Minimize
	dst.Type = v1beta1.MetricType(src.Type)
	dst.Query = src.Query
	dst.ErrorQuery = src.Query
	dst.Scheme = src.Scheme
	dst.Selector = src.Selector
	dst.Port = src.Port
	dst.Path = src.Path
	return nil
}

func (dst *Metric) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.Metric)
	dst.Name = src.Name
	dst.Minimize = src.Minimize
	dst.Type = MetricType(src.Type)
	dst.Query = src.Query
	dst.ErrorQuery = src.Query
	dst.Scheme = src.Scheme
	dst.Selector = src.Selector
	dst.Port = src.Port
	dst.Path = src.Path
	return nil
}

func (src *PatchReadinessGate) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.PatchReadinessGate)
	dst.ConditionType = src.ConditionType
	return nil
}

func (dst *PatchReadinessGate) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.PatchReadinessGate)
	dst.ConditionType = src.ConditionType
	return nil
}
func (src *PatchTemplate) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.PatchTemplate)
	dst.Type = v1beta1.PatchType(src.Type)
	dst.Patch = src.Patch
	dst.TargetRef = src.TargetRef
	if src.ReadinessGates != nil {
		in, out := &src.ReadinessGates, &dst.ReadinessGates
		*out = make([]v1beta1.PatchReadinessGate, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (dst *PatchTemplate) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.PatchTemplate)
	dst.Type = PatchType(src.Type)
	dst.Patch = src.Patch
	dst.TargetRef = src.TargetRef
	if src.ReadinessGates != nil {
		in, out := &src.ReadinessGates, &dst.ReadinessGates
		*out = make([]PatchReadinessGate, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (src *NamespaceTemplateSpec) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.NamespaceTemplateSpec)
	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	return nil
}

func (dst *NamespaceTemplateSpec) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.NamespaceTemplateSpec)
	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = src.Spec
	return nil
}

func (src *TrialTemplateSpec) ConvertTo(dstRaw controller.Hub) error {
	dst := dstRaw.(*v1beta1.TrialTemplateSpec)
	dst.ObjectMeta = src.ObjectMeta
	if err := src.Spec.convertTo(&dst.Spec); err != nil {
		return err
	}
	return nil
}

func (dst *TrialTemplateSpec) ConvertFrom(srcRaw controller.Hub) error {
	src := srcRaw.(*v1beta1.TrialTemplateSpec)
	dst.ObjectMeta = src.ObjectMeta
	if err := dst.Spec.convertFrom(&src.Spec); err != nil {
		return err
	}
	return nil
}

func (src *Experiment) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.Experiment)
	dst.ObjectMeta = src.ObjectMeta
	dst.Spec.Replicas = src.Spec.Replicas
	if src.Spec.Optimization != nil {
		in, out := &src.Spec.Optimization, &dst.Spec.Optimization
		*out = make([]v1beta1.Optimization, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	if src.Spec.Parameters != nil {
		in, out := &src.Spec.Parameters, &dst.Spec.Parameters
		*out = make([]v1beta1.Parameter, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	if src.Spec.Constraints != nil {
		in, out := &src.Spec.Constraints, &dst.Spec.Constraints
		*out = make([]v1beta1.Constraint, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	if src.Spec.Metrics != nil {
		in, out := &src.Spec.Metrics, &dst.Spec.Metrics
		*out = make([]v1beta1.Metric, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	if src.Spec.Patches != nil {
		in, out := &src.Spec.Patches, &dst.Spec.Patches
		*out = make([]v1beta1.PatchTemplate, len(*in))
		for i := range *in {
			if err := (*in)[i].ConvertTo(&(*out)[i]); err != nil {
				return err
			}
		}
	}
	dst.Spec.NamespaceSelector = src.Spec.NamespaceSelector
	if src.Spec.NamespaceTemplate != nil {
		in, out := &src.Spec.NamespaceTemplate, &dst.Spec.NamespaceTemplate
		*out = new(v1beta1.NamespaceTemplateSpec)
		if err := (*in).ConvertTo(*out); err != nil {
			return err
		}
	}
	if err := src.Spec.Template.ConvertTo(&dst.Spec.Template); err != nil {
		return err
	}
	dst.Status.Phase = src.Status.Phase
	dst.Status.ActiveTrials = src.Status.ActiveTrials
	return nil
}

func (dst *Experiment) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.Experiment)
	dst.ObjectMeta = src.ObjectMeta
	dst.Spec.Replicas = src.Spec.Replicas
	if src.Spec.Optimization != nil {
		in, out := &src.Spec.Optimization, &dst.Spec.Optimization
		*out = make([]Optimization, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	if src.Spec.Parameters != nil {
		in, out := &src.Spec.Parameters, &dst.Spec.Parameters
		*out = make([]Parameter, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	if src.Spec.Constraints != nil {
		in, out := &src.Spec.Constraints, &dst.Spec.Constraints
		*out = make([]Constraint, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	if src.Spec.Metrics != nil {
		in, out := &src.Spec.Metrics, &dst.Spec.Metrics
		*out = make([]Metric, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	if src.Spec.Patches != nil {
		in, out := &src.Spec.Patches, &dst.Spec.Patches
		*out = make([]PatchTemplate, len(*in))
		for i := range *in {
			if err := (*out)[i].ConvertFrom(&(*in)[i]); err != nil {
				return err
			}
		}
	}
	dst.Spec.NamespaceSelector = src.Spec.NamespaceSelector
	if src.Spec.NamespaceTemplate != nil {
		in, out := &src.Spec.NamespaceTemplate, &dst.Spec.NamespaceTemplate
		*out = new(NamespaceTemplateSpec)
		if err := (*out).ConvertFrom(*in); err != nil {
			return err
		}
	}
	if err := dst.Spec.Template.ConvertFrom(&src.Spec.Template); err != nil {
		return err
	}
	dst.Status.Phase = src.Status.Phase
	dst.Status.ActiveTrials = src.Status.ActiveTrials
	return nil
}
