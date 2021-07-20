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
)

func ClusterExperimentToAPITemplate(exp *optimizev1beta2.Experiment) (*applications.Template, error) {
	template := &applications.Template{}

	// TODO need to handle baselines
	// This gets handled differently in experiments vs applications
	params := parameters(exp)
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(paramBytes, &template.Parameters); err != nil {
		return nil, err
	}

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

func APITemplateToClusterExperiment(exp *optimizev1beta2.Experiment, templ *applications.Template) error {
	// TODO

	return nil
}
