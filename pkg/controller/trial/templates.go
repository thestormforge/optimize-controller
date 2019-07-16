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
package trial

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type patchContext struct {
	Trial  patchTrial
	Values map[string]int64
}

type patchTrial struct {
	Name string
}

type metricContext struct {
	// The time at which the trial run started (possibly adjusted)
	StartTime time.Time
	// The time at which the trial run completed
	CompletionTime time.Time
	// The duration of the trial run expressed as a Prometheus range value
	Range string
}

func executePatchTemplate(p *redskyv1alpha1.PatchTemplate, trial *redskyv1alpha1.Trial) (types.PatchType, []byte, error) {
	// Determine the patch type
	var patchType types.PatchType
	switch p.Type {
	case "json":
		patchType = types.JSONPatchType
	case "merge":
		patchType = types.MergePatchType
	case "strategic", "":
		patchType = types.StrategicMergePatchType
	default:
		return "", nil, fmt.Errorf("unknown patch type: %s", p.Type)
	}

	// Execute the patch and ensure the result is JSON (not YAML)
	data, err := executeAssignmentTemplate(p.Patch, trial)
	if err != nil {
		return "", nil, err
	}
	if data == nil {
		return patchType, data, nil
	}
	json, err := yaml.ToJSON(data)
	return patchType, json, err
}

func executeAssignmentTemplate(t string, trial *redskyv1alpha1.Trial) ([]byte, error) {
	// Create the functions map
	funcMap := template.FuncMap{
		"percent": templatePercent,
	}

	// Create the data context
	data := patchContext{}
	data.Trial = patchTrial{Name: trial.Name}
	data.Values = make(map[string]int64, len(trial.Spec.Assignments))
	for _, a := range trial.Spec.Assignments {
		data.Values[a.Name] = a.Value
	}

	// Evaluate the template into a patch
	tmpl, err := template.New("patch").Funcs(funcMap).Parse(t)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	if err = tmpl.Execute(buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func executeMetricQueryTemplate(m *redskyv1alpha1.Metric, trial *redskyv1alpha1.Trial) (string, error) {
	// Create the functions map
	funcMap := template.FuncMap{
		"duration": templateDuration,
	}

	// Create the data context
	data := metricContext{}
	if trial.Status.StartTime != nil {
		data.StartTime = trial.Status.StartTime.Time
	}
	if trial.Status.CompletionTime != nil {
		data.CompletionTime = trial.Status.CompletionTime.Time
	}
	data.Range = fmt.Sprintf("%.0fs", templateDuration(data.StartTime, data.CompletionTime))

	// Evaluate the template into a query
	tmpl, err := template.New("query").Funcs(funcMap).Parse(m.Query)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	if err = tmpl.Execute(buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// TODO Should we use http://masterminds.github.io/sprig/

func templateDuration(start, completion time.Time) float64 {
	if start.Before(completion) {
		return completion.Sub(start).Seconds()
	}
	return 0
}

func templatePercent(value int64, percent int64) string {
	return fmt.Sprintf("%d", int64(float64(value)*(float64(percent)/100.0)))
}
