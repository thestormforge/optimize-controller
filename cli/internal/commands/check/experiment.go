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
	"context"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/internal/experiment"
	"github.com/thestormforge/optimize-controller/v2/internal/template"
	"github.com/thestormforge/optimize-controller/v2/internal/validation"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
)

// Define linter log levels
// NOTE: It is unclear why zapr is reversing the sign of the level.
const (
	vError = int(-zapcore.ErrorLevel)
	vWarn  = int(-zapcore.WarnLevel)
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
		RunE:   commander.WithContextE(o.checkExperiment),
	}

	cmd.Flags().StringVarP(&o.Filename, "filename", "f", "", "`file` that contains the experiment to check")

	_ = cmd.MarkFlagFilename("filename", "yml", "yaml")
	_ = cmd.MarkFlagRequired("filename")

	return cmd
}

func (o *ExperimentOptions) checkExperiment(ctx context.Context) error {
	r, err := o.IOStreams.OpenFile(o.Filename)
	if err != nil {
		return err
	}

	// Unmarshal the experiment
	exp := &optimizev1beta2.Experiment{}
	rr := commander.NewResourceReader()
	if err := rr.ReadInto(r, exp); err != nil {
		return err
	}

	// Create a new linter for traversing the experiment and reporting errors
	l := &linter{}

	// Set the recommended budget to 20x the number of parameters up to 400 trials
	l.minExperimentBudget = 20 * len(exp.Spec.Parameters)
	if l.minExperimentBudget > 400 {
		l.minExperimentBudget = 400
	}

	// Create a zapr logger for reporting issues
	// NOTE: We are using logr and zap because that is what controller-runtime uses
	var hasError bool
	l.logger = zapr.NewLogger(zap.New(zapcore.NewCore(zapcore.NewConsoleEncoder(
		zapcore.EncoderConfig{
			MessageKey:  "msg",
			LevelKey:    "level",
			EncodeLevel: zapcore.LowercaseColorLevelEncoder,
		}),
		zapcore.AddSync(o.ErrOut),
		zapcore.WarnLevel),
		zap.Hooks(func(e zapcore.Entry) error {
			if e.Level == zapcore.ErrorLevel {
				hasError = true
			}
			return nil
		})))

	// Use the linter to inspect the experiment
	experiment.Walk(ctx, l, exp)

	// TODO Ideally we would just return an error here, but it would look strange alongside the other output
	if hasError {
		os.Exit(1)
	}

	return nil
}

type linter struct {
	logger logr.Logger

	// The minimum recommended value for the experiment budget optimization parameter.
	minExperimentBudget int
}

func (l *linter) Visit(ctx context.Context, obj interface{}) experiment.Visitor {
	// Add the current path to the logger
	lint := l.logger.WithValues("path", strings.Join(experiment.WalkPath(ctx), "/"))

	switch o := obj.(type) {

	case *optimizev1beta2.Optimization:
		switch o.Name {
		case "experimentBudget":
			if eb, err := strconv.Atoi(o.Value); err != nil {
				lint.Error(err, "Optimization parameter value must be an integer", "value", o.Value)
			} else if l.minExperimentBudget > 0 && l.minExperimentBudget > eb {
				lint.V(vWarn).Info("Experiment budget should be increased", "experimentBudget", eb, "recommended", l.minExperimentBudget)
			}
		}

	case []optimizev1beta2.Parameter:
		if l := len(o); l == 0 {
			lint.V(vError).Info("Parameters are required")
		} else if b := countBaselines(o); b > 0 && b != l {
			lint.V(vError).Info("Baseline must be specified on all parameters")
		}

	case []optimizev1beta2.Metric:
		if len(o) == 0 {
			lint.V(vError).Info("Metrics are required")
		}

	case *optimizev1beta2.Parameter:
		if len(o.Values) > 0 && (o.Min != 0 || o.Max != 0) {
			// NOTE: This won't hit on v1alpha1 converted experiments because min/max get reset
			lint.V(vWarn).Info("Parameter has both a numeric and string range defined")
		} else if o.Max <= o.Min && (o.Max != 0 || o.Min != 0) {
			lint.V(vError).Info("Parameter minimum must be strictly less then maximum", "min", o.Min, "max", o.Max)
		} else if o.Baseline != nil {
			checkBaseline(lint, o)
		}

	case *optimizev1beta2.Metric:
		switch o.Type {
		case
			optimizev1beta2.MetricKubernetes,
			optimizev1beta2.MetricPrometheus,
			optimizev1beta2.MetricJSONPath,
			optimizev1beta2.MetricDatadog,
			"": // Type is valid
		default:
			lint.V(vError).Info("Metric type is invalid", "type", o.Type)
		}

		if o.Query == "" {
			lint.V(vError).Info("Metric query is required")
		} else {
			q, _, err := metricQueryDryRun(o)
			if err != nil {
				lint.Error(err, "Metric query failed to render", "query", o.Query)
			}

			switch o.Type {
			case optimizev1beta2.MetricJSONPath:
				if !strings.Contains(q, "{") {
					lint.V(vWarn).Info("JSON Path query should contain an {} expression", "query", o.Query)
				}
			case optimizev1beta2.MetricPrometheus:
				if !strings.Contains(q, "scalar") {
					lint.V(vWarn).Info("Prometheus query may require explicit scalar conversion", "query", o.Query)
				}
			}
		}

		if o.Min != nil && o.Max != nil && o.Min.Cmp(*o.Max) <= 0 {
			lint.V(vError).Info("Metric minimum must be strictly less then maximum")
		}

		if u, err := url.Parse(o.URL); err != nil {
			lint.V(vError).Info("Metric has invalid URL")
		} else if u.Hostname() == "redskyops.dev" {
			lint.V(vError).Info("Metric requires manual conversion to latest version for URL")
		}

	case *optimizev1beta2.PatchTemplate:
		if o.TargetRef != nil {
			if o.TargetRef.Kind == "" {
				// TODO Is kind required? Can you just have the namespace and the rest of the ref in the patch?
				lint.V(vError).Info("Patch target kind is required")
			} else if _, ok := scheme.Scheme.AllKnownTypes()[o.TargetRef.GroupVersionKind()]; !ok {
				if o.TargetRef.APIVersion == "" {
					lint.V(vError).Info("Patch target apiVersion is required")
				}
				if o.Type == optimizev1beta2.PatchStrategic || o.Type == "" {
					lint.V(vWarn).Info("Strategic merge patch may not work with custom resources")
				}
			}
		}

		if _, err := template.New().RenderPatch(o, &optimizev1beta2.Trial{}); err != nil {
			lint.Error(err, "Patch is not valid")
		}

	case *batchv1beta1.JobTemplateSpec:
		if o.Spec.BackoffLimit != nil && *o.Spec.BackoffLimit != 0 {
			lint.V(vWarn).Info("Job backoffLimit should be 0", "backoffLimit", *o.Spec.BackoffLimit)
		}

	}

	// Return the linter to continue walking through the experiment
	return l
}

func countBaselines(params []optimizev1beta2.Parameter) int {
	var b int
	for i := range params {
		if params[i].Baseline != nil {
			b++
		}
	}
	return b
}

func checkBaseline(lint logr.Logger, p *optimizev1beta2.Parameter) {
	switch p.Baseline.Type {
	case intstr.String:
		if p.Min != 0 || p.Max != 0 {
			lint.V(vError).Info("Parameter defines a numeric range but has a string baseline value")
		} else if len(p.Values) == 0 {
			lint.V(vError).Info("Parameter has a string baseline but no values")
		} else if !validation.CheckParameterValue(p, *p.Baseline) {
			lint.V(vError).Info("Parameter baseline is not in range", "values", strings.Join(p.Values, ","), "baseline", p.Baseline.StrVal)
		}

	case intstr.Int:
		if len(p.Values) > 0 {
			lint.V(vError).Info("Parameter defines a string range but has a numeric baseline value")
		} else if p.Min == 0 && p.Max == 0 {
			lint.V(vError).Info("Parameter has a numeric baseline but no min or max")
		} else if p.Min != p.Max && !validation.CheckParameterValue(p, *p.Baseline) {
			lint.V(vError).Info("Parameter baseline is not in range", "min", p.Min, "max", p.Max, "baseline", p.Baseline.IntVal)
		}
	}
}

func metricQueryDryRun(m *optimizev1beta2.Metric) (string, string, error) {
	// Try to dummy out the target object to avoid failures
	target := &unstructured.Unstructured{}
	if m.Target != nil {
		target.SetGroupVersionKind(m.Target.GroupVersionKind())
	}

	return template.New().RenderMetricQueries(m, &optimizev1beta2.Trial{}, target)
}
