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

package metric

import (
	"context"
	"fmt"
	"strconv"
	"time"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/internal/metric/prometheus"
	"github.com/redskyops/redskyops-controller/internal/template"
	"k8s.io/apimachinery/pkg/runtime"
)

// Source represents a source of metric data
type Source interface {
	// Capture queries a metric source for a value
	Capture(ctx context.Context) (value float64, stddev float64, err error)
}

// Input represents all of the information available when capturing metrics
type Input struct {
	// The name of the metric being captured
	Name string
	// The configuration URL for the metric
	MetricURL URL
	// The beginning of the capture window
	StartTime time.Time
	// The end of the capture window
	CompletionTime time.Time
	// The metric value query
	Query string
	// The metric standard deviation query
	ErrorQuery string

	// An input also acts as metric URL resolver, i.e. specific to a target runtime object
	Resolver
}

// CaptureError describes problems that arise while capturing metric values
type CaptureError struct {
	// A description of what went wrong
	Message string
	// The URL that was used to capture the metric
	Address string
	// The metric query that failed
	Query string
	// The completion time at which the query was executed
	CompletionTime time.Time
	// The minimum amount of time until the metric is expected to be available
	RetryAfter time.Duration
}

func (e *CaptureError) Error() string {
	return e.Message
}

// Not sure what the approriate constructor looks like for this yet
// Initially I was thinking we'd have Input contain everything, but in it's current state
// Input relies on generated data that we handle internally. So we can change
// it to be similar to the Capture call below --  (ctx, trial, target) or look at
// changing around how Input is used.
func NewSource(mt redskyv1beta1.MetricType, input *Input) (Source, error) {
	// Handle all of the parsing/translation that needs to be done from the metric definition

	// Execute the query as a template against the current state of the trial
	if in.Query, in.ErrorQuery, err = template.New().RenderMetricQueries(metric, trial, target); err != nil {
		return value, stddev, err
	}

	// Parse the metric URL
	if in.MetricURL, err = ParseURL(metric.URL); err != nil {
		return value, stddev, err
	}

	// Handle resolution of input MetricURL
	targets := lookupURLs(input.MetricURL, NewResolver(target))

	switch mt {

	case redskyv1beta1.MetricLocal, redskyv1beta1.MetricPods, "":
		return sourceFunc(captureLocalMetric), nil

	case redskyv1beta1.MetricPrometheus:
		if len(targets) != 1 {
			return nil, fmt.Errorf("only a single prometheus server is supported, we discovered %d", len(targets))
		}

		// NewCollector implements the Source interface
		// The source interface definition changes from Capture(ctx, input) to Capture(ctx)
		// to prevent cyclical imports.
		return prometheus.NewCollector(
			targets[0],
			in.Name,
			in.Query,
			in.ErrorQuery,
			in.StartTime,
			in.CompleteTime,
		)

	case redskyv1beta1.MetricDatadog:
		return sourceFunc(captureDatadogMetric), nil

	case redskyv1beta1.MetricJSONPath:
		return sourceFunc(captureJSONPathMetric), nil

	default:
		return nil, fmt.Errorf("unknown metric type: %s", mt)
	}
}

// Capture captures a point-in-time metric value and it's error (standard deviation)
func Capture(ctx context.Context, metric *redskyv1beta1.Metric, trial *redskyv1beta1.Trial, target runtime.Object) (value float64, stddev float64, err error) {
	// Create a new metric capture input
	in := &Input{
		Name:           metric.Name,
		StartTime:      trial.Status.StartTime.Time,
		CompletionTime: trial.Status.CompletionTime.Time,
		Resolver:       NewResolver(target),
	}

	// Execute the query as a template against the current state of the trial
	if in.Query, in.ErrorQuery, err = template.New().RenderMetricQueries(metric, trial, target); err != nil {
		return value, stddev, err
	}

	// Parse the metric URL
	if in.MetricURL, err = ParseURL(metric.URL); err != nil {
		return value, stddev, err
	}

	// Find the metric source and use it to capture the metric value
	src, err := FindSource(metric.Type)
	if err != nil {
		return value, stddev, err
	}
	return src.Capture(ctx, in)
}

// TODO We can probably clean this up later and make the sources actual types (especially for types which make use of a reusable client)

type sourceFunc func(ctx context.Context, in *Input) (value float64, stddev float64, err error)

func (sf sourceFunc) Capture(ctx context.Context, in *Input) (value float64, stddev float64, err error) {
	return sf(ctx, in)
}

func captureLocalMetric(_ context.Context, in *Input) (value float64, stddev float64, err error) {
	value, err = strconv.ParseFloat(in.Query, 64)
	// TODO Parse the error query if it is not empty?
	return value, stddev, err
}

// lookupURLs returns a list of real URLs given a metric URL and a resolver
func lookupURLs(u *URL, r Resolver) ([]string, error) {
	a, err := r.LookupAuthority(u.Scheme, u.Host, u.Port)
	if err != nil {
		return nil, err
	}
	urls := make([]string, len(a))
	for i := range a {
		uu := u.newURL()
		uu.Host = a[i]
		urls[i] = uu.String()
	}
	return urls, nil
}
