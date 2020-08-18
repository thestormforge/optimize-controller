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
	"io/ioutil"
	"strings"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/internal/metric"
	"github.com/redskyops/redskyops-controller/internal/template"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"
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

	cmd.Flags().StringVarP(&o.Filename, "filename", "f", "", "File that contains the experiment to check.")

	_ = cmd.MarkFlagFilename("filename", "yml", "yaml")

	commander.ExitOnError(cmd)
	return cmd
}

func (o *ExperimentOptions) checkExperiment() error {
	// Read the entire input
	var data []byte
	var err error
	if o.Filename == "-" {
		data, err = ioutil.ReadAll(o.In)
	} else {
		data, err = ioutil.ReadFile(o.Filename)
	}
	if err != nil {
		return err
	}

	// Unmarshal the experiment
	experiment := &redskyv1beta1.Experiment{}
	if err = yaml.Unmarshal(data, experiment); err != nil {
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

	for i := range parameters {
		checkParameter(lint.For(i), &parameters[i])
	}

}

func checkParameter(lint Linter, parameter *redskyv1beta1.Parameter) {

}

func checkMetrics(lint Linter, metrics []redskyv1beta1.Metric) {

	if len(metrics) == 0 {
		lint.Error().Missing("metrics")
	}

	for i := range metrics {
		checkMetric(lint.For(i), &metrics[i])
	}

}

func checkMetric(lint Linter, m *redskyv1beta1.Metric) {

	if m.Query == "" {
		lint.Error().Missing("query")
	}

	u, err := metric.ParseURL(m.URL)
	if err != nil {
		lint.Error().Failed("url", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		lint.Error().Invalid("url.scheme", u.Scheme, "http", "https")
	}

	q, _, err := template.New().RenderMetricQueries(m, &redskyv1beta1.Trial{}, nil)
	if err != nil {
		lint.Error().Failed("query", err)
	}

	// JSON path queries must have at least one curley brace
	if m.Type == redskyv1beta1.MetricJSONPath && !strings.Contains(q, "{") {
		lint.Error().Invalid("query", m.Query)
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
