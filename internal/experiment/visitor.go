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

package experiment

import (
	"context"
	"fmt"

	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
)

// Visitor is used to inspect individual sections of an experiment.
type Visitor interface {
	// Visit an experiment section. The supplied object will be a slice or a pointer
	// to a type on the experiment. The return value is used to halt traversal.
	Visit(ctx context.Context, obj interface{}) Visitor
}

// Walk traverses an experiment depth first; obj must not be nil; visitor will be invoked
// with relevant non-nil members of the experiment followed by an invocation with nil.
func Walk(ctx context.Context, v Visitor, obj interface{}) {
	if v = v.Visit(ctx, obj); v == nil {
		return
	}

	switch o := obj.(type) {

	case *optimizev1beta2.Experiment:
		Walk(withPath(ctx, "spec"), v, &o.Spec)

	case *optimizev1beta2.ExperimentSpec:
		Walk(withPath(ctx, "optimization"), v, o.Optimization)
		Walk(withPath(ctx, "parameters"), v, o.Parameters)
		Walk(withPath(ctx, "metrics"), v, o.Metrics)
		Walk(withPath(ctx, "patches"), v, o.Patches)
		Walk(withPath(ctx, "trialTemplate"), v, &o.TrialTemplate)

	case []optimizev1beta2.Optimization:
		for i := range o {
			Walk(withPath(ctx, map[string]string{"name": o[i].Name}), v, &o[i])
		}

	case *optimizev1beta2.Optimization:
		// Do nothing

	case []optimizev1beta2.Parameter:
		for i := range o {
			Walk(withPath(ctx, map[string]string{"name": o[i].Name}), v, &o[i])
		}

	case *optimizev1beta2.Parameter:
		// Do nothing

	case []optimizev1beta2.Metric:
		for i := range o {
			Walk(withPath(ctx, map[string]string{"name": o[i].Name}), v, &o[i])
		}

	case *optimizev1beta2.Metric:
		// Do nothing

	case []optimizev1beta2.PatchTemplate:
		for i := range o {
			Walk(withPath(ctx, i), v, &o[i])
		}

	case *optimizev1beta2.PatchTemplate:
		// Do nothing

	case *optimizev1beta2.TrialTemplateSpec:
		if o.Spec.JobTemplate != nil {
			Walk(withPath(withPath(ctx, "spec"), "jobTemplate"), v, o.Spec.JobTemplate)
		}

	case *batchv1beta1.JobTemplateSpec:
		// Do nothing

	default:
		panic(fmt.Sprintf("experiment.Walk: unexpected type %T", obj))
	}

	v.Visit(ctx, nil)
}

// pathKey is used as a context key for the walk path.
type pathKey struct{}

// WalkPath returns the path to current element on the context as an array of element names.
func WalkPath(ctx context.Context) []string {
	switch v := ctx.Value(pathKey{}).(type) {
	case []string:
		return v
	default:
		return []string{""}
	}
}

// withPath adds the specified elements to the path while walking.
func withPath(ctx context.Context, elem interface{}) context.Context {
	if ctx == nil {
		return nil
	}

	path := make([]string, 0, 6)
	path = append(path, WalkPath(ctx)...)

	switch e := elem.(type) {
	case string:
		path = append(path, e)
	case int:
		path = append(path, fmt.Sprintf("%d", e))
	case map[string]string:
		for k, v := range e {
			path = append(path, fmt.Sprintf("[%s=%s]", k, v))
		}
	}

	return context.WithValue(ctx, pathKey{}, path)
}
