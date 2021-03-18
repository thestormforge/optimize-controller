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

package generation

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/application"
	"github.com/yujunz/go-getter"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func init() {
	// Hack the sort order used by the format filter to make experiments sort more naturally
	addFieldOrder := func(obj interface{}, order int) {
		t := reflect.Indirect(reflect.ValueOf(obj)).Type()
		for i := 0; i < t.NumField(); i++ {
			if tag := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]; tag != "" {
				if _, ok := yaml.FieldOrder[tag]; !ok {
					yaml.FieldOrder[tag] = order
				}
				order++
			}
		}
	}

	addFieldOrder(&redskyv1beta1.ExperimentSpec{}, 200)
	addFieldOrder(&redskyv1beta1.Parameter{}, 300)
	addFieldOrder(&redskyv1beta1.PatchTemplate{}, 400)
	addFieldOrder(&redskyv1beta1.Metric{}, 500)

	// TODO We should probably move this code to a more generic YAML package
	addFieldOrder(&redskyappsv1alpha1.Application{}, 600)
}

// newGoalMetric creates a new metric for the supplied goal with most fields pre-filled.
func newGoalMetric(obj *redskyappsv1alpha1.Goal, query string) redskyv1beta1.Metric {
	defer func() { obj.Implemented = true }()
	return redskyv1beta1.Metric{
		Type:     redskyv1beta1.MetricPrometheus,
		Query:    query,
		Minimize: true,
		Name:     obj.Name,
		Min:      obj.Min,
		Max:      obj.Max,
		Optimize: obj.Optimize,
	}
}

// ensureTrialJobPod returns the pod template for the trial job, creating the job template if necessary.
func ensureTrialJobPod(exp *redskyv1beta1.Experiment) *corev1.PodTemplateSpec {
	if exp.Spec.TrialTemplate.Spec.JobTemplate == nil {
		exp.Spec.TrialTemplate.Spec.JobTemplate = &batchv1beta1.JobTemplateSpec{}
	}
	return &exp.Spec.TrialTemplate.Spec.JobTemplate.Spec.Template
}

// splitPath splits a string based path, honoring backslash escaped slashes.
func splitPath(p string) []string {
	// TODO This is using the Kustomize API, refactor it out
	return (&types.FieldSpec{Path: p}).PathSlice()
}

// trialJobImage returns the image name for a type of job.
func trialJobImage(job string) string {
	// Allow the image name to be overridden using environment variables, primarily for development work
	imageName := os.Getenv("OPTIMIZE_TRIALS_IMAGE_REPOSITORY")
	if imageName == "" {
		imageName = "thestormforge/optimize-trials"
	}
	imageTag := os.Getenv("OPTIMIZE_TRIALS_IMAGE_TAG")
	if imageTag == "" {
		imageTagBase := os.Getenv("OPTIMIZE_TRIALS_IMAGE_TAG_BASE")
		if imageTagBase == "" {
			imageTagBase = "v0.0.1"
		}
		imageTag = imageTagBase + "-" + job
	}
	return imageName + ":" + imageTag
}

// loadApplicationData loads data (e.g. a supporting test file).
func loadApplicationData(app *redskyappsv1alpha1.Application, src string) ([]byte, error) {
	dst := filepath.Join(os.TempDir(), fmt.Sprintf("load-application-data-%x", md5.Sum([]byte(src))))
	defer os.Remove(dst)

	// Only set the working directory to directory of the file the app.yaml was loaded from,
	// we MUST NOT set this to the process working directory or relative paths in the app.yaml
	// will be dependent on what directory you run the process from. If the path annotation is
	// not present on the application, we MUST fail to load relative paths.
	opts := []getter.ClientOption{
		func(c *getter.Client) error {
			c.Pwd = application.WorkingDirectory(app)
			return nil
		},
	}

	if err := getter.GetFile(dst, src, opts...); err != nil {
		// TODO We need to be better about wrapping errors with more context here
		return nil, fmt.Errorf("unable to load file: %w", err)
	}

	return ioutil.ReadFile(dst)
}

var metricSelectorPattern = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)(=|!=|=~|!~)"([a-zA-Z0-9\-|]+)"`)

// convertPrometheusSelector converts a Prometheus metric selector to a Kubernetes
// label selector. This is necessary because objectives like "Requests" define their
// "selector" as a Prometheus selector: in order for that to work with a metric
// with `type: kubernetes` (or `type: ""`), we must first convert it over.
func convertPrometheusSelector(metricSelector string) (*metav1.LabelSelector, error) {
	if metricSelector == "" {
		return nil, nil
	}

	labelSelector := &metav1.LabelSelector{}
	for _, ms := range strings.Split(metricSelector, ",") {
		parts := metricSelectorPattern.FindStringSubmatch(ms)
		if len(parts) != 4 {
			return nil, fmt.Errorf("invalid metric selector")
		}

		switch parts[2] {
		case "=":
			if labelSelector.MatchLabels == nil {
				labelSelector.MatchLabels = make(map[string]string)
			}
			labelSelector.MatchLabels[parts[1]] = parts[3]
		case "!=":
			labelSelector.MatchExpressions = append(labelSelector.MatchExpressions, metav1.LabelSelectorRequirement{
				Key:      parts[1],
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   []string{parts[3]},
			})
		case "=~":
			labelSelector.MatchExpressions = append(labelSelector.MatchExpressions, metav1.LabelSelectorRequirement{
				Key:      parts[1],
				Operator: metav1.LabelSelectorOpIn,
				Values:   strings.Split(parts[3], "|"),
			})
		case "!~":
			labelSelector.MatchExpressions = append(labelSelector.MatchExpressions, metav1.LabelSelectorRequirement{
				Key:      parts[1],
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   strings.Split(parts[3], "|"),
			})
		}
	}

	return labelSelector, nil
}
