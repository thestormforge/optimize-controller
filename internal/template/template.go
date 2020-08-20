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

package template

import (
	"bytes"
	"fmt"
	"math"
	"text/template"
	"time"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// PatchData represents a trial during patch evaluation
type PatchData struct {
	// Trial metadata
	Trial metav1.ObjectMeta
	// Trial assignments
	Values map[string]int64
}

// MetricData represents a trial during metric evaluation
type MetricData struct {
	// Trial metadata
	Trial metav1.ObjectMeta
	// The time at which the trial run started (possibly adjusted)
	StartTime time.Time
	// The time at which the trial run completed
	CompletionTime time.Time
	// The duration of the trial run expressed as a Prometheus range value
	Range string
	// Trial assignments
	Values map[string]int64
	// List of pods from the trial namespace (only available for "pods" type metrics)
	Pods *corev1.PodList
}

func newPatchData(t *redskyv1beta1.Trial) *PatchData {
	d := &PatchData{}

	t.ObjectMeta.DeepCopyInto(&d.Trial)

	d.Values = make(map[string]int64, len(t.Spec.Assignments))
	for _, a := range t.Spec.Assignments {
		d.Values[a.Name] = a.Value
	}

	return d
}

func newMetricData(t *redskyv1beta1.Trial, target runtime.Object) *MetricData {
	d := &MetricData{}

	t.ObjectMeta.DeepCopyInto(&d.Trial)

	d.Values = make(map[string]int64, len(t.Spec.Assignments))
	for _, a := range t.Spec.Assignments {
		d.Values[a.Name] = a.Value
	}

	if pods, ok := target.(*corev1.PodList); ok {
		d.Pods = pods
	}

	if t.Status.StartTime != nil {
		d.StartTime = t.Status.StartTime.Time
	}

	if t.Status.CompletionTime != nil {
		d.CompletionTime = t.Status.CompletionTime.Time
	}

	d.Range = fmt.Sprintf("%.0fs", math.Max(d.CompletionTime.Sub(d.StartTime).Seconds(), 0))

	return d
}

// Engine is used to render Go text templates
type Engine struct {
	FuncMap template.FuncMap
}

// New creates a new template engine
func New() *Engine {
	return &Engine{
		FuncMap: FuncMap(),
	}
}

// TODO Investigate better use of template names
// Would it be possible to have the template engine hold more scope? e.g. create the template engine using the full list
// of patch templates or metrics (or the experiment itself, trial for HelmValues) and then render the individual values by template name?

// RenderPatch returns the JSON representation of the supplied patch template (input can be a Go template that produces YAML)
func (e *Engine) RenderPatch(patch *redskyv1beta1.PatchTemplate, trial *redskyv1beta1.Trial) ([]byte, error) {
	data := newPatchData(trial)
	b, err := e.render("patch", patch.Patch, data) // TODO What should we use for patch template names? Something from the targetRef?
	if err != nil {
		return nil, err
	}
	return yaml.ToJSON(b.Bytes())
}

// RenderHelmValue returns a rendered string of the supplied Helm value
func (e *Engine) RenderHelmValue(helmValue *redskyv1beta1.HelmValue, trial *redskyv1beta1.Trial) (string, error) {
	data := newPatchData(trial)
	b, err := e.render(helmValue.Name, helmValue.Value.String(), data)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

// RenderMetricQueries returns the metric query and the metric error query
func (e *Engine) RenderMetricQueries(metric *redskyv1beta1.Metric, trial *redskyv1beta1.Trial, target runtime.Object) (string, string, error) {
	data := newMetricData(trial, target)
	b1, err := e.render(metric.Name, metric.Query, data)
	if err != nil {
		return "", "", err
	}
	b2, err := e.render(metric.Name, metric.ErrorQuery, data)
	if err != nil {
		return "", "", err
	}
	return b1.String(), b2.String(), nil
}

func (e *Engine) render(name, text string, data interface{}) (*bytes.Buffer, error) {
	tmpl, err := template.New(name).Funcs(e.FuncMap).Parse(text)
	if err != nil {
		return nil, err
	}

	b := &bytes.Buffer{}
	if err = tmpl.Execute(b, data); err != nil {
		return nil, err
	}
	return b, nil
}
