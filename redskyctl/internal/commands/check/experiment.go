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

package check

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/template"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commander"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
)

// ExperimentOptions are the options for checking an experiment manifest
type ExperimentOptions struct {
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	Filename string
}

// NewExperimentCommand creates a new command for checking an experiment manifest
func NewExperimentCommand(o *ExperimentOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "experiment",
		Short: "Check an experiment",
		Long:  "Check an experiment manifest",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithoutArgsE(o.checkExperiment),
	}

	cmd.Flags().StringVarP(&o.Filename, "filename", "f", "", "file that contains the experiment to check")

	_ = cmd.MarkFlagFilename("filename", "yml", "yaml")
	_ = cmd.MarkFlagRequired("filename")

	return cmd
}

func (o *ExperimentOptions) checkExperiment() error {
	r, err := o.IOStreams.OpenFile(o.Filename)
	if err != nil {
		return err
	}

	// Unmarshal the experiment
	experiment := &redskyv1beta1.Experiment{}
	rr := commander.NewResourceReader()
	if err := rr.ReadInto(r, experiment); err != nil {
		return err
	}

	// Check that everything looks right
	linter := &AllTheLint{}
	checkExperiment(linter.For("experiment"), experiment)

	// Share the results
	// TODO Filter/sort?
	for _, p := range linter.Problems {
		_, _ = fmt.Fprintln(o.Out, p.Message)
	}

	return nil
}

func checkExperiment(lint Linter, experiment *redskyv1beta1.Experiment) {

	if !checkTypeMeta(lint.For("metadata"), &experiment.TypeMeta) {
		return
	}

	checkParameters(lint.For("spec", "parameters"), experiment.Spec.Parameters)
	checkMetrics(lint.For("spec", "metrics"), experiment.Spec.Metrics)
	checkPatches(lint.For("spec", "patches"), experiment.Spec.Patches)
	checkTrialTemplate(lint.For("spec", "template"), &experiment.Spec.TrialTemplate)

	// TODO Some checks are higher level and need a combination of pieces: e.g. selector/template matching

}

func checkTypeMeta(lint Linter, typeMeta *metav1.TypeMeta) bool {
	// TODO Should we have a "fatal" severity (i.e. -1) instead of trying to keep track of "ok"?
	ok := true

	if typeMeta.Kind != "Experiment" {
		lint.For("metadata").Error().Invalid("kind", typeMeta.Kind, "Experiment")
		ok = false
	}

	if typeMeta.APIVersion != redskyv1beta1.GroupVersion.String() {
		lint.For("metadata").Error().Invalid("apiVersion", typeMeta.APIVersion, redskyv1beta1.GroupVersion.String())
		ok = false
	}

	return ok
}

func checkParameters(lint Linter, parameters []redskyv1beta1.Parameter) {

	if len(parameters) == 0 {
		lint.Error().Missing("parameters")
	}

	var baseline int
	for i := range parameters {
		checkParameter(lint.For(i), &parameters[i])
		if parameters[i].Baseline != nil {
			baseline++
		}
	}

	if baseline > 0 && baseline != len(parameters) {
		lint.Warning().Missing("baseline: should be on all parameters or none")
	}

}

func checkParameter(lint Linter, parameter *redskyv1beta1.Parameter) {

	if parameter.Baseline != nil {
		if parameter.Baseline.Type == intstr.String {
			if parameter.Min > 0 || parameter.Max > 0 {
				lint.For().Error().Invalid("baseline", parameter.Baseline, "<number>")
			} else if len(parameter.Values) > 0 {
				var allowed []interface{}
				for _, v := range parameter.Values {
					if parameter.Baseline.StrVal != v {
						allowed = append(allowed, v)
					}
				}
				if len(allowed) == len(parameter.Values) {
					lint.For().Error().Invalid("baseline", parameter.Baseline, allowed...)
				}
			}
		} else {
			if len(parameter.Values) > 0 {
				lint.For().Error().Invalid("baseline", parameter.Baseline, parameter.Values)
			} else if parameter.Min != parameter.Max {
				if parameter.Baseline.IntVal < parameter.Min {
					lint.For().Error().Invalid("baseline", parameter.Baseline, fmt.Sprintf("<greater than %d>", parameter.Min))
				}
				if parameter.Baseline.IntVal > parameter.Max {
					lint.For().Error().Invalid("baseline", parameter.Baseline, fmt.Sprintf("<less than %d>", parameter.Max))
				}
			}
		}
	}

}

func checkMetrics(lint Linter, metrics []redskyv1beta1.Metric) {

	if len(metrics) == 0 {
		lint.Error().Missing("metrics")
	}

	for i := range metrics {
		checkMetric(lint.For(i), &metrics[i])
	}

}

func checkMetric(lint Linter, metric *redskyv1beta1.Metric) {

	if metric.Query == "" {
		lint.Error().Missing("query")
	}

	if metric.Type == redskyv1beta1.MetricPrometheus && metric.Selector == nil {
		lint.Error().Missing("selector for Prometheus metric")
	}

	if metric.Type == redskyv1beta1.MetricJSONPath {
		// TODO We need to render the template first
		if !strings.Contains(metric.Query, "{") {
			lint.Error().Invalid("query", metric.Query)
		}
	}

	if metric.Scheme != "" && strings.ToLower(metric.Scheme) == "http" && strings.ToLower(metric.Scheme) != "https" {
		lint.Error().Invalid("scheme", metric.Scheme, "http", "https")
	}

	if _, _, err := template.New().RenderMetricQueries(metric, &redskyv1beta1.Trial{}, nil); err != nil {
		lint.Error().Failed("query", err)
	}

}

func checkPatches(lint Linter, patches []redskyv1beta1.PatchTemplate) {

	if len(patches) == 0 {
		lint.Error().Missing("patches")
	}

	for i := range patches {
		checkPatch(lint.For(i), &patches[i])
	}

}

func checkPatch(lint Linter, patch *redskyv1beta1.PatchTemplate) {

	if patch.TargetRef.APIVersion == "" {
		// TODO Is is OK to skip this for the core kinds or should we still require "v1"?
		if !isCoreKind(patch.TargetRef.Kind) {
			lint.Error().Missing("API version")
		}
	}

	if patch.TargetRef.Kind == "" {
		lint.Error().Missing("kind")
	}

	if _, err := template.New().RenderPatch(patch, &redskyv1beta1.Trial{}); err != nil {
		lint.Error().Failed("patch", err)
	}

}

func checkTrialTemplate(lint Linter, template *redskyv1beta1.TrialTemplateSpec) {
	checkTrial(lint.For("spec"), &template.Spec)
}

func checkTrial(lint Linter, trial *redskyv1beta1.TrialSpec) {
	if trial.JobTemplate != nil {
		checkJobTemplate(lint.For("jobTemplate"), trial.JobTemplate)
	}
}

func checkJobTemplate(lint Linter, template *v1beta1.JobTemplateSpec) {
	checkJob(lint.For("spec"), &template.Spec)
}

func checkJob(lint Linter, job *batchv1.JobSpec) {
	if job.BackoffLimit != nil && *job.BackoffLimit != 0 {
		// TODO Instead of "Invalid" can we have "Suggested"?
		lint.Warning().Invalid("backoffLimit", *job.BackoffLimit, 0)
	}
}

// Check if a kind is one of the known core types
func isCoreKind(kind string) bool {
	for coreKind := range scheme.Scheme.KnownTypes(schema.GroupVersion{Version: "v1"}) {
		if coreKind == kind {
			return true
		}
	}
	return false
}
