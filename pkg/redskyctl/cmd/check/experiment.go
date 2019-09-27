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

package check

import (
	"fmt"
	"io/ioutil"
	"strings"

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	"github.com/redskyops/k8s-experiment/pkg/controller/template"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"
)

const (
	checkExperimentLong    = `Check an experiment manifest`
	checkExperimentExample = ``
)

type CheckExperimentOptions struct {
	Filename string

	cmdutil.IOStreams
}

func NewCheckExperimentOptions(ioStreams cmdutil.IOStreams) *CheckExperimentOptions {
	return &CheckExperimentOptions{
		IOStreams: ioStreams,
	}
}

func NewCheckExperimentCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewCheckExperimentOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "experiment",
		Short:   "Check an experiment",
		Long:    checkExperimentLong,
		Example: checkExperimentExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVarP(&o.Filename, "filename", "f", "", "File that contains the experiment to check.")

	return cmd
}

func (o *CheckExperimentOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	return nil
}

func (o *CheckExperimentOptions) Run() error {
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

	// TODO Can we use the REST API to send a dry-run for validation?

	// Unmarshal the experiment and check it
	var problems []LintError
	experiment := &redskyv1alpha1.Experiment{}
	if err = yaml.Unmarshal(data, experiment); err != nil {
		problems = []LintError{{Message: err.Error()}}
	} else {
		problems = CheckExperiment(experiment)
	}

	// Share the results
	// TODO Filter/sort?
	for _, p := range problems {
		_, _ = fmt.Fprintln(o.Out, p.Error())
	}

	return nil
}

func CheckExperiment(experiment *redskyv1alpha1.Experiment) []LintError {
	lint := NewLinter()

	checkParameters(lint.For("spec", "parameters"), experiment.Spec.Parameters)
	checkMetrics(lint.For("spec", "metrics"), experiment.Spec.Metrics)
	checkPatches(lint.For("spec", "patches"), experiment.Spec.Patches)
	checkTrialTemplate(lint.For("spec", "template"), &experiment.Spec.Template)

	// TODO Some checks are higher level and need a combination of pieces: e.g. selector/template matching

	return lint.Problems
}

func checkParameters(lint Linter, parameters []redskyv1alpha1.Parameter) {

	if len(parameters) == 0 {
		lint.Error().Missing("parameters")
	}

	for i := range parameters {
		checkParameter(lint.For(i), &parameters[i])
	}

}

func checkParameter(lint Linter, parameter *redskyv1alpha1.Parameter) {

}

func checkMetrics(lint Linter, metrics []redskyv1alpha1.Metric) {

	if len(metrics) == 0 {
		lint.Error().Missing("metrics")
	}

	for i := range metrics {
		checkMetric(lint.For(i), &metrics[i])
	}

}

func checkMetric(lint Linter, metric *redskyv1alpha1.Metric) {

	if metric.Query == "" {
		lint.Error().Missing("query")
	}

	if metric.Type == redskyv1alpha1.MetricPrometheus && metric.Selector == nil {
		lint.Error().Missing("selector for Prometheus metric")
	}

	if metric.Scheme != "" && strings.ToLower(metric.Scheme) == "http" && strings.ToLower(metric.Scheme) != "https" {
		lint.Error().Invalid("scheme", metric.Scheme, "http", "https")
	}

	if _, _, err := template.NewTemplateEngine().RenderMetricQueries(metric, &redskyv1alpha1.Trial{}); err != nil {
		lint.Error().Failed("query", err)
	}

}

func checkPatches(lint Linter, patches []redskyv1alpha1.PatchTemplate) {

	if len(patches) == 0 {
		lint.Error().Missing("patches")
	}

	for i := range patches {
		checkPatch(lint.For(i), &patches[i])
	}

}

func checkPatch(lint Linter, patch *redskyv1alpha1.PatchTemplate) {

	if patch.TargetRef.APIVersion == "" {
		// TODO Is is OK to skip this for the core kinds or should we still require "v1"?
		if !isCoreKind(patch.TargetRef.Kind) {
			lint.Error().Missing("API version")
		}
	}

	if patch.TargetRef.Kind == "" {
		lint.Error().Missing("kind")
	}

	if _, err := template.NewTemplateEngine().RenderPatch(patch, &redskyv1alpha1.Trial{}); err != nil {
		lint.Error().Failed("patch", err)
	}

}

func checkTrialTemplate(lint Linter, template *redskyv1alpha1.TrialTemplateSpec) {
	checkTrial(lint.For("spec"), &template.Spec)
}

func checkTrial(lint Linter, trial *redskyv1alpha1.TrialSpec) {

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
