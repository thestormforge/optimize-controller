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

package server

import (
	"encoding/json"

	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	applications "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func ClusterExperimentToAPITemplate(exp *optimizev1beta2.Experiment) (*applications.Template, error) {
	template := &applications.Template{}

	params := parameters(exp)

	bases, err := baselines(exp)
	if err != nil {
		return nil, err
	}

	combinedParams, err := combineParamAndBaseline(params, bases)
	if err != nil {
		return nil, err
	}

	template.Parameters = combinedParams

	metric := metrics(exp)
	metricBytes, err := json.Marshal(metric)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(metricBytes, &template.Metrics); err != nil {
		return nil, err
	}

	return template, nil
}

func APITemplateToClusterExperiment(exp *optimizev1beta2.Experiment, template *applications.Template) error {
	if exp == nil || template == nil {
		return nil
	}

	p, err := apiParamsToClusterParams(template.Parameters)
	if err != nil {
		return err
	}

	exp.Spec.Parameters = p

	// We only allow modifying/overwriting existing metrics by the same name
	// and we only support changing Optimize, Bounds(Min/Max), Minimize
	for m := range exp.Spec.Metrics {
		for tm := range template.Metrics {
			if template.Metrics[tm].Name != exp.Spec.Metrics[m].Name {
				continue
			}

			exp.Spec.Metrics[m].Minimize = template.Metrics[tm].Minimize
			exp.Spec.Metrics[m].Optimize = template.Metrics[tm].Optimize

			if template.Metrics[tm].Bounds == nil {
				continue
			}

			exp.Spec.Metrics[m].Min = resource.NewQuantity(int64(template.Metrics[tm].Bounds.Min), resource.DecimalSI)
			exp.Spec.Metrics[m].Max = resource.NewQuantity(int64(template.Metrics[tm].Bounds.Max), resource.DecimalSI)

		}
	}

	return nil
}

func apiParamsToClusterParams(applicationParams []applications.TemplateParameter) ([]optimizev1beta2.Parameter, error) {
	cp := make([]optimizev1beta2.Parameter, 0, len(applicationParams))

	for _, ap := range applicationParams {
		param := optimizev1beta2.Parameter{
			Name: ap.Name,
		}

		switch ap.Type {
		case "categorical":
			param.Values = ap.Values
		case "int":
			min, err := ap.Bounds.Min.Int64()
			if err != nil {
				return nil, err
			}

			max, err := ap.Bounds.Max.Int64()
			if err != nil {
				return nil, err
			}

			param.Min = int32(min)
			param.Max = int32(max)
		}

		if ap.Baseline != nil {
			baseline := intstr.FromString(ap.Baseline.String())
			param.Baseline = &baseline
		}

		cp = append(cp, param)
	}

	return cp, nil
}

func combineParamAndBaseline(params []experimentsv1alpha1.Parameter, baselines []experimentsv1alpha1.Assignment) ([]applications.TemplateParameter, error) {

	combined := make([]applications.TemplateParameter, 0, len(params))

	for _, param := range params {
		// Marshal / Unmarshal dance(?)
		expParamBytes, err := json.Marshal(param)
		if err != nil {
			return nil, err
		}

		appTemplate := applications.TemplateParameter{}

		if err := json.Unmarshal(expParamBytes, &appTemplate); err != nil {
			return nil, err
		}

		for _, baseline := range baselines {
			if baseline.ParameterName != param.Name {
				continue
			}

			appTemplate.Baseline = &baseline.Value
		}

		combined = append(combined, appTemplate)
	}

	return combined, nil
}
