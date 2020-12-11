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
	"net/url"
	"strings"

	"github.com/thestormforge/optimize-controller/api/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/util/intstr"
	conv "sigs.k8s.io/controller-runtime/pkg/conversion"
)

const (
	// LegacyHostnamePlaceholder is a special hostname (that should never, ever occur in practice) used to mark
	// URLs which have been generated from legacy `Service` selectors. When the controller encounters this hostname
	// it should be replaced by the resolved service's cluster IP address (if applicable) or name.
	LegacyHostnamePlaceholder = "redskyops.dev"
)

var _ conv.Convertible = &Experiment{}

func (in *Experiment) ConvertTo(hub conv.Hub) error {
	s, err := SchemeBuilder.Build()
	if err != nil {
		return err
	}
	return s.Convert(in, hub.(*v1beta1.Experiment), nil)
}

func (in *Experiment) ConvertFrom(hub conv.Hub) error {
	s, err := SchemeBuilder.Build()
	if err != nil {
		return err
	}
	return s.Convert(hub.(*v1beta1.Experiment), in, nil)
}

func Convert_v1alpha1_ExperimentSpec_To_v1beta1_ExperimentSpec(in *ExperimentSpec, out *v1beta1.ExperimentSpec, s conversion.Scope) error {
	// Rename `Template` to `TrialTemplate`
	if err := Convert_v1alpha1_TrialTemplateSpec_To_v1beta1_TrialTemplateSpec(&in.Template, &out.TrialTemplate, s); err != nil {
		return err
	}

	// Continue
	return autoConvert_v1alpha1_ExperimentSpec_To_v1beta1_ExperimentSpec(in, out, s)
}

func Convert_v1beta1_ExperimentSpec_To_v1alpha1_ExperimentSpec(in *v1beta1.ExperimentSpec, out *ExperimentSpec, s conversion.Scope) error {
	// Rename `TrialTemplate` to `Template`
	if err := Convert_v1beta1_TrialTemplateSpec_To_v1alpha1_TrialTemplateSpec(&in.TrialTemplate, &out.Template, s); err != nil {
		return err
	}

	// Continue
	return autoConvert_v1beta1_ExperimentSpec_To_v1alpha1_ExperimentSpec(in, out, s)
}

func Convert_v1alpha1_Parameter_To_v1beta1_Parameter(in *Parameter, out *v1beta1.Parameter, s conversion.Scope) error {
	err := autoConvert_v1alpha1_Parameter_To_v1beta1_Parameter(in, out, s)
	if err != nil {
		return err
	}

	// v1beta1 cannot have values and bounds
	if len(in.Values) > 0 {
		out.Min = 0
		out.Max = 0
	}

	return nil
}

func Convert_v1beta1_Parameter_To_v1alpha1_Parameter(in *v1beta1.Parameter, out *Parameter, s conversion.Scope) error {
	err := autoConvert_v1beta1_Parameter_To_v1alpha1_Parameter(in, out, s)
	if err != nil {
		return err
	}

	// v1alpha1 didn't have categorical (it's only there for round tripping) enforce it via a max
	if len(in.Values) > 0 {
		out.Max = int32(len(in.Values) - 1)
	}

	return nil
}

func Convert_v1beta1_Metric_To_v1alpha1_Metric(in *v1beta1.Metric, out *Metric, s conversion.Scope) error {
	if err := autoConvert_v1beta1_Metric_To_v1alpha1_Metric(in, out, s); err != nil {
		return err
	}

	u, err := url.Parse(in.URL)
	if err != nil {
		return err
	}
	out.Scheme, out.Port, out.Path = fromURL(in, u)

	switch in.Type {
	case v1beta1.MetricKubernetes:
		if in.Target == nil || in.Target.Kind == "" {
			out.Type = "local"
		} else if in.Target.Kind == "Pod" && in.Target.APIVersion == "v1" {
			out.Type = "pods"
		}
	}

	if in.Target != nil {
		out.Selector = in.Target.LabelSelector
	}

	return nil
}

func Convert_v1alpha1_Metric_To_v1beta1_Metric(in *Metric, out *v1beta1.Metric, s conversion.Scope) error {
	if err := autoConvert_v1alpha1_Metric_To_v1beta1_Metric(in, out, s); err != nil {
		return err
	}

	out.URL = toURL(in)

	switch in.Type {
	case "local":
		out.Type = v1beta1.MetricKubernetes

	case "pods":
		out.Type = v1beta1.MetricKubernetes
		out.Target = &v1beta1.ResourceTarget{
			Kind:       "Pod",
			APIVersion: "v1",
		}
	}

	if in.Selector != nil {
		if out.Target == nil {
			out.Target = &v1beta1.ResourceTarget{}
		}
		out.Target.LabelSelector = in.Selector
	}

	return nil
}

func fromURL(m *v1beta1.Metric, u *url.URL) (scheme string, port intstr.IntOrString, path string) {
	if m.Type == v1beta1.MetricDatadog {
		scheme = u.Query().Get("aggregator")
		return scheme, port, path
	}

	if u.Scheme != "http" {
		scheme = u.Scheme
	}

	if p := u.Port(); p != "" {
		port = intstr.Parse(p)
	} else if m.Type == v1beta1.MetricPrometheus && m.Target != nil && isBuiltInPrometheusSelector(m.Target.LabelSelector) {
		port = intstr.FromInt(9090)
	}

	path = u.Path
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}

	return scheme, port, path
}

func toURL(m *Metric) string {
	switch m.Type {

	case "prometheus":
		// Allow the default labels to convert over to the default URL
		if m.Selector == nil || isBuiltInPrometheusSelector(m.Selector) {
			return ""
		}

	case "datadog":
		// The DataDog URL can only come from `DATADOG_HOST` so just use the URL to hold the "aggregator"
		q := url.Values{"aggregator": []string{m.Scheme}}
		return (&url.URL{RawQuery: q.Encode()}).String()

	case "jsonpath":
		// Nothing extra to do here

	default:
		// Ignore types that are not URL based
		return ""
	}

	// Use a special placeholder for the host
	u := url.URL{
		Scheme: m.Scheme,
		Host:   LegacyHostnamePlaceholder,
	}

	// Completely ignore named ports
	if m.Port.IntValue() > 0 {
		u.Host += ":" + m.Port.String()
	}

	// Include legacy default for scheme
	if u.Scheme == "" {
		u.Scheme = "http"
	}

	// Legacy path included query parameters
	q := strings.SplitN(m.Path, "?", 2)
	u.Path = q[0]
	if len(q) > 1 {
		u.RawQuery = q[1]
	}

	return u.String()
}

func isBuiltInPrometheusSelector(s *metav1.LabelSelector) bool {
	if s == nil || len(s.MatchLabels) != 1 {
		return false
	}
	return s.MatchLabels["app"] == "prometheus"
}
