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

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/yujunz/go-getter"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func init() {
	// Hack the sort order used by the format filter to make experiments sort more naturally
	yaml.FieldOrder["parameters"] = 100
	yaml.FieldOrder["metrics"] = 200
	yaml.FieldOrder["targetRef"] = 100
	yaml.FieldOrder["patch"] = 200
	yaml.FieldOrder["baseline"] = 100
	yaml.FieldOrder["min"] = 200
	yaml.FieldOrder["max"] = 300
}

// newObjectiveMetric creates a new metric for the supplied objective with most fields pre-filled.
func newObjectiveMetric(obj *redskyappsv1alpha1.Objective, query string) redskyv1beta1.Metric {
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

	opts := []getter.ClientOption{
		func(c *getter.Client) error {
			// Try to load relative to the application definition itself
			if path := app.Annotations["config.kubernetes.io/path"]; path != "" {
				c.Pwd = filepath.Dir(path)
			}
			return nil
		},
	}

	if err := getter.GetFile(dst, src, opts...); err != nil {
		return nil, fmt.Errorf("unable to load Locust file: %w", err)
	}

	return ioutil.ReadFile(dst)
}
