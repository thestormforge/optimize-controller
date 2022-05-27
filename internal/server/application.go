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
	"fmt"

	"github.com/thestormforge/konjure/pkg/konjure"
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	applications "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// APIApplicationToClusterApplication converts an API (optimize-go) representation
// of an Application into an in cluster ( api/apps/v1alpha1 ) representation.
func APIApplicationToClusterApplication(app applications.Application, scenario applications.Scenario) (*optimizeappsv1alpha1.Application, error) {
	if err := validateAPIApplication(app, scenario); err != nil {
		return nil, err
	}

	// Construct a controller representation of an application from the api definition
	baseApp := &optimizeappsv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: app.Name.String(),
		},
	}

	resources, err := apiResources(app)
	if err != nil {
		return nil, err
	}

	baseApp.Resources = resources

	params, err := apiParameters(scenario)
	if err != nil {
		return nil, err
	}

	baseApp.Configuration = params

	objectives, err := apiObjectives(scenario)
	if err != nil {
		return nil, err
	}

	baseApp.Objectives = objectives

	scenarios, err := apiScenarios(scenario)
	if err != nil {
		return nil, err
	}

	baseApp.Scenarios = scenarios

	return baseApp, nil
}

// ClusterExperimentToAPITemplate converts an Application into an in cluster
// ( api/apps/v1alpha1 ) representation into an API (optimize-go) representation.
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

	// Template metrics have additional bounds information that must be propagated
	for i := range exp.Spec.Metrics {
		m := &exp.Spec.Metrics[i]
		var b *applications.TemplateMetricBounds
		if m.Max != nil || m.Min != nil {
			b = &applications.TemplateMetricBounds{}
			if m.Max != nil {
				b.Max = float64(m.Max.MilliValue()) / 1000
			}
			if m.Min != nil {
				b.Min = float64(m.Min.MilliValue()) / 1000
			}
		}
		template.Metrics = append(template.Metrics, applications.TemplateMetric{
			Name:     m.Name,
			Minimize: m.Minimize,
			Optimize: m.Optimize,
			Bounds:   b,
		})
	}

	return template, nil
}

// APITemplateToClusterExperiment overlays the results of an application template
// ( parameters, metrics ) on top of an existing experiment.
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

			if ap.Baseline != nil {
				baseline := intstr.FromString(ap.Baseline.String())
				param.Baseline = &baseline
			}
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

			if ap.Baseline != nil {
				baseline := intstr.FromInt(int(ap.Baseline.Int64Value()))
				param.Baseline = &baseline
			}
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

		for b := range baselines {
			if baselines[b].ParameterName != param.Name {
				continue
			}

			appTemplate.Baseline = &baselines[b].Value
		}

		combined = append(combined, appTemplate)
	}

	return combined, nil
}

func validateAPIApplication(app applications.Application, scenario applications.Scenario) error {
	if app.Name == "" {
		return fmt.Errorf("invalid application, missing name")
	}

	if len(app.Resources) == 0 {
		return fmt.Errorf("invalid application, no resources specified")
	}

	if len(scenario.Objective) == 0 {
		return fmt.Errorf("invalid scenario, no objectives specified")
	}

	if len(scenario.Configuration) == 0 {
		return fmt.Errorf("invalid scenario, no configuration specified")
	}

	return nil
}

func apiResources(app applications.Application) (konjure.Resources, error) {
	data, err := json.Marshal(app.Resources)
	if err != nil {
		return nil, err
	}

	var resources konjure.Resources

	if err := json.Unmarshal(data, &resources); err != nil {
		return nil, err
	}

	return resources, nil
}

func apiParameters(scenario applications.Scenario) ([]optimizeappsv1alpha1.Parameter, error) {
	// Parameters
	rawParams, err := json.Marshal(scenario.Configuration)
	if err != nil {
		return nil, err
	}

	params := []optimizeappsv1alpha1.Parameter{}
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	return params, nil
}

func apiObjectives(scenario applications.Scenario) ([]optimizeappsv1alpha1.Objective, error) {
	rawObjectives, err := json.Marshal(scenario.Objective)
	if err != nil {
		return nil, err
	}

	goals := []optimizeappsv1alpha1.Goal{}
	if err := json.Unmarshal(rawObjectives, &goals); err != nil {
		return nil, err
	}

	objectives := []optimizeappsv1alpha1.Objective{{Goals: goals}}

	return objectives, nil
}

func apiScenarios(scenario applications.Scenario) ([]optimizeappsv1alpha1.Scenario, error) {
	data, err := json.Marshal(scenario)
	if err != nil {
		return nil, err
	}

	appScenario := optimizeappsv1alpha1.Scenario{}

	if err = json.Unmarshal(data, &appScenario); err != nil {
		return nil, err
	}

	return []optimizeappsv1alpha1.Scenario{appScenario}, nil
}
