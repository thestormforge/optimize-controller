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
	"context"
	"fmt"
	"path"
	"strings"

	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/sfio"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type StormForgePerformanceSource struct {
	Scenario    *optimizeappsv1alpha1.Scenario
	Objective   *optimizeappsv1alpha1.Objective
	Application *optimizeappsv1alpha1.Application
}

var _ ExperimentSource = &StormForgePerformanceSource{} // Update trial job
var _ MetricSource = &StormForgePerformanceSource{}     // StormForge specific metrics
var _ kio.Reader = &StormForgePerformanceSource{}       // ConfigMap for the test case file

func (s *StormForgePerformanceSource) Update(exp *optimizev1beta2.Experiment) error {
	if s.Scenario == nil || s.Application == nil {
		return nil
	}

	pod := &ensureTrialJobPod(exp).Spec
	pod.Containers = []corev1.Container{
		{
			Name:  s.Scenario.Name,
			Image: trialJobImage("stormforge-perf"),
			Env: []corev1.EnvVar{
				{
					Name: "TITLE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
				{
					Name:  "TEST_CASE",
					Value: s.Scenario.StormForge.TestCase,
				},
			},
		},
	}

	// The test case file can be blank, in which case it must be uploaded to StormForge Performance ahead of time
	if testCaseFile := s.testCaseFile(); testCaseFile != "" {
		pod.Containers[0].Env = append(pod.Containers[0].Env, corev1.EnvVar{
			Name:  "TEST_CASE_FILE",
			Value: testCaseFile,
		})
		pod.Containers[0].VolumeMounts = append(pod.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "test-case-file",
			ReadOnly:  true,
			MountPath: path.Dir(testCaseFile),
		})
		pod.Volumes = append(pod.Volumes, corev1.Volume{
			Name: "test-case-file",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: s.testCaseFileConfigMapName(),
					},
				},
			},
		})
	}

	// TODO We need to rethink how ingress scanning works, this just preserves existing behavior
	var ingressURL string
	if s.Application != nil && s.Application.Ingress != nil {
		ingressURL = s.Application.Ingress.URL
	}
	if ingressURL != "" {
		if !strings.Contains(ingressURL, ".") {
			return fmt.Errorf("ingress should be fully qualified when using StormForge scenarios")
		}
		pod.Containers[0].Env = append(pod.Containers[0].Env, corev1.EnvVar{Name: "TARGET", Value: ingressURL})
	}

	// Add a reference to the access token
	pod.Containers[0].Env = append(pod.Containers[0].Env, corev1.EnvVar{
		Name: "STORMFORGER_JWT",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: s.accessTokenSecretName(),
				},
				Key: "STORMFORGER_JWT",
			},
		},
	})

	return nil
}

func (s *StormForgePerformanceSource) Read() ([]*yaml.RNode, error) {
	result := sfio.ObjectSlice{}

	// Get the test case file path and the access token we need for generating the configuration resources
	testCaseFile := s.testCaseFile()
	accessToken, err := s.accessToken()
	if err != nil {
		return nil, err
	}

	// Include a secret with the access token
	secret := &corev1.Secret{}
	secret.Name = s.accessTokenSecretName()
	secret.Data = map[string][]byte{"STORMFORGER_JWT": []byte(accessToken)}
	result = append(result, secret)

	// If there is a test case file, create a ConfigMap for it
	if testCaseFile != "" {
		data, err := loadApplicationData(s.Application, s.Scenario.StormForge.TestCaseFile)
		if err != nil {
			return nil, err
		}

		cm := &corev1.ConfigMap{}
		cm.Name = s.testCaseFileConfigMapName()
		cm.Data = map[string]string{path.Base(testCaseFile): string(data)}
		result = append(result, cm)
	}

	return result.Read()
}

func (s *StormForgePerformanceSource) Metrics() ([]optimizev1beta2.Metric, error) {
	var result []optimizev1beta2.Metric
	if s.Objective == nil {
		return result, nil
	}

	for i := range s.Objective.Goals {
		goal := &s.Objective.Goals[i]
		switch {

		case goal.Implemented:
			// Do nothing

		case goal.Latency != nil:
			if l := s.stormForgePerfLatency(goal.Latency.LatencyType); l != "" {
				query := `scalar(` + l + `{job="trialRun",instance="{{ .Trial.Name }}"})`
				result = append(result, newGoalMetric(goal, query))
			}

		case goal.ErrorRate != nil:
			if goal.ErrorRate.ErrorRateType == optimizeappsv1alpha1.ErrorRateRequests {
				query := `scalar(error_ratio{job="trialRun",instance="{{ .Trial.Name }}"})`
				result = append(result, newGoalMetric(goal, query))
			}

		}
	}
	return result, nil
}

// testCaseFileConfigMapName is the name to use for the config map that holds
// the test case definition (JavaScript).
func (s *StormForgePerformanceSource) testCaseFileConfigMapName() string {
	return "stormforge-perf-test-case"
}

// accessTokenSecretName returns the name to use for the secret that holds the
// access token (STORMFORGER_JWT).
func (s *StormForgePerformanceSource) accessTokenSecretName() string {
	return "stormforge-perf-access-token"
}

// serviceAccountLabel returns the label applied to Performance Service Account
// associated with the access token (when using service accounts).
func (s *StormForgePerformanceSource) serviceAccountLabel() string {
	return fmt.Sprintf("optimize-%s", s.Application.Name)
}

// testCaseFile returns the path to use for `TEST_CASE_FILE`.
func (s *StormForgePerformanceSource) testCaseFile() string {
	// NOTE: The `s.Scenario.StormForge.TestCaseFile` might be a URL or even
	// inline JavaScript content; it should NOT be used to make the file name

	// If there is no test case file information, return an empty path
	if s.Scenario.StormForge.TestCaseFile == "" {
		return ""
	}

	// Make sure we strip the optional organization prefix from the test case name
	_, testCase := splitTestCase(s.Scenario.StormForge.TestCase)
	return "/forge-init.d/" + testCase + ".js"
}

// accessToken returns the value to use for `STORMFORGER_JWT`.
func (s *StormForgePerformanceSource) accessToken() (string, error) {
	ctx := context.Background()
	org, _ := splitTestCase(s.Scenario.StormForge.TestCase)

	// Hide the bodies.
	token, err := (&StormForgePerformanceAuthorization{ServiceAccountLabel: s.serviceAccountLabel}).AccessToken(ctx, org)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(token, "\n"), nil
}

// stormForgePerfLatency normalizes the latency enumeration to match the StormForge Performance values.
func (s *StormForgePerformanceSource) stormForgePerfLatency(lt optimizeappsv1alpha1.LatencyType) string {
	switch optimizeappsv1alpha1.FixLatency(lt) {
	case optimizeappsv1alpha1.LatencyMinimum:
		return "min"
	case optimizeappsv1alpha1.LatencyMaximum:
		return "max"
	case optimizeappsv1alpha1.LatencyMean:
		return "mean"
	case optimizeappsv1alpha1.LatencyPercentile50:
		return "median"
	case optimizeappsv1alpha1.LatencyPercentile95:
		return "percentile_95"
	case optimizeappsv1alpha1.LatencyPercentile99:
		return "percentile_99"
	default:
		return ""
	}
}

// splitTestCase splits the supplied string into an optional organization and test case name.
func splitTestCase(s string) (org string, testCase string) {
	parts := strings.SplitN(s, "/", 2)
	switch len(parts) {
	case 1:
		testCase = parts[0]
	case 2:
		org = parts[0]
		testCase = parts[1]
	}
	return
}
