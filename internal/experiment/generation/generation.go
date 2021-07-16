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

	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/application"
	"github.com/yujunz/go-getter"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// newGoalMetric creates a new metric for the supplied goal with most fields pre-filled.
func newGoalMetric(obj *optimizeappsv1alpha1.Goal, query string) optimizev1beta2.Metric {
	defer func() { obj.Implemented = true }()
	return optimizev1beta2.Metric{
		Type:     optimizev1beta2.MetricPrometheus,
		Query:    query,
		Minimize: true,
		Name:     obj.Name,
		Min:      obj.Min,
		Max:      obj.Max,
		Optimize: obj.Optimize,
	}
}

// ensureTrialJobPod returns the pod template for the trial job, creating the job template if necessary.
func ensureTrialJobPod(exp *optimizev1beta2.Experiment) *corev1.PodTemplateSpec {
	if exp.Spec.TrialTemplate.Spec.JobTemplate == nil {
		exp.Spec.TrialTemplate.Spec.JobTemplate = &batchv1beta1.JobTemplateSpec{}
	}
	return &exp.Spec.TrialTemplate.Spec.JobTemplate.Spec.Template
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
func loadApplicationData(app *optimizeappsv1alpha1.Application, src string) ([]byte, error) {
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
