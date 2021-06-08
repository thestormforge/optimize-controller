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
	"fmt"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml"
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/sfio"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type StormForgerSource struct {
	Scenario    *optimizeappsv1alpha1.Scenario
	Objective   *optimizeappsv1alpha1.Objective
	Application *optimizeappsv1alpha1.Application
}

var _ ExperimentSource = &StormForgerSource{} // Update trial job
var _ MetricSource = &StormForgerSource{}     // StormForger specific metrics
var _ kio.Reader = &StormForgerSource{}       // ConfigMap for the test case file

func (s *StormForgerSource) Update(exp *optimizev1beta2.Experiment) error {
	if s.Scenario == nil || s.Application == nil {
		return nil
	}

	org, tc := s.stormForgerTestCase()
	if org == "" {
		return fmt.Errorf("missing StormForger organization")
	}

	accessToken := s.stormForgerAccessToken(org)
	if accessToken == nil {
		return fmt.Errorf("missing StormForger authorization")
	}

	pod := &ensureTrialJobPod(exp).Spec
	pod.Containers = []corev1.Container{
		{
			Name:  "stormforger",
			Image: trialJobImage("stormforger"),
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
					Value: fmt.Sprintf("%s/%s", org, tc),
				},
			},
		},
	}

	// The test case file can be blank, in which case it must be uploaded to StormForger ahead of time
	if s.Scenario.StormForger.TestCaseFile != "" {
		pod.Containers[0].VolumeMounts = append(pod.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "test-case-file",
			ReadOnly:  true,
			MountPath: "/forge-init.d",
		})
		pod.Containers[0].Env = append(pod.Containers[0].Env, corev1.EnvVar{
			Name:  "TEST_CASE_FILE",
			Value: "/forge-init.d/" + tc + ".js",
		})
		pod.Volumes = append(pod.Volumes, corev1.Volume{
			Name: "test-case-file",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: s.stormForgerConfigMapName(),
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
			return fmt.Errorf("ingress should be fully qualified when using StormForger scenarios")
		}
		pod.Containers[0].Env = append(pod.Containers[0].Env, corev1.EnvVar{Name: "TARGET", Value: ingressURL})
	}

	// Add a reference to the access token
	pod.Containers[0].Env = append(pod.Containers[0].Env, corev1.EnvVar{
		Name: "STORMFORGER_JWT",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: accessToken.SecretKeyRef,
		},
	})

	return nil
}

func (s *StormForgerSource) Read() ([]*yaml.RNode, error) {
	result := sfio.ObjectSlice{}

	org, tc := s.stormForgerTestCase()

	// If there is a test case file, create a ConfigMap for it
	if s.Scenario.StormForger.TestCaseFile != "" {
		data, err := loadApplicationData(s.Application, s.Scenario.StormForger.TestCaseFile)
		if err != nil {
			return nil, err
		}

		cm := &corev1.ConfigMap{}
		cm.Name = s.stormForgerConfigMapName()
		cm.Data = map[string]string{tc + ".js": string(data)}
		result = append(result, cm)
	}

	// Include a secret with the access token, if necessary
	if accessToken := s.stormForgerAccessToken(org); accessToken != nil {
		secret := &corev1.Secret{}
		secret.Name = accessToken.SecretKeyRef.Name
		switch {
		case accessToken.File != "":
			data, err := loadApplicationData(s.Application, accessToken.File)
			if err != nil {
				return nil, err
			}
			secret.Data = map[string][]byte{accessToken.SecretKeyRef.Key: data}
			result = append(result, secret)

		case accessToken.Literal != "":
			secret.Data = map[string][]byte{accessToken.SecretKeyRef.Key: []byte(accessToken.Literal)}
			result = append(result, secret)
		}
	}

	return result.Read()
}

func (s *StormForgerSource) Metrics() ([]optimizev1beta2.Metric, error) {
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
			if l := s.stormForgerLatency(goal.Latency.LatencyType); l != "" {
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

func (s *StormForgerSource) stormForgerTestCase() (org, name string) {
	parts := strings.Split(s.Scenario.StormForger.TestCase, "/")
	if len(parts) == 2 {
		org = parts[0]
		name = parts[1]
	} else {
		name = parts[0]
	}

	if org == "" && s.Application.StormForger != nil {
		org = s.Application.StormForger.Organization
	}

	if name == "" {
		name = fmt.Sprintf("%s-%s", s.Application.Name, s.Scenario.Name)
	}

	return
}

func (s *StormForgerSource) stormForgerConfigMapName() string {
	return fmt.Sprintf("%s-test-case-file", s.Scenario.Name)
}

// stormForgerAccessToken returns the effective access token information.
func (s *StormForgerSource) stormForgerAccessToken(org string) *optimizeappsv1alpha1.StormForgerAccessToken {
	// This helper function ensures we return something with a populated secret key ref
	fixRef := func(accessToken *optimizeappsv1alpha1.StormForgerAccessToken) *optimizeappsv1alpha1.StormForgerAccessToken {
		if accessToken.SecretKeyRef == nil {
			accessToken.SecretKeyRef = &corev1.SecretKeySelector{}
		}

		if accessToken.SecretKeyRef.Name == "" {
			accessToken.SecretKeyRef.Name = optimizeappsv1alpha1.StormForgerAccessTokenSecretName
		}

		if accessToken.SecretKeyRef.Key == "" {
			accessToken.SecretKeyRef.Key = org
		}

		return accessToken
	}

	// Use the access token specified in the application
	if s.Application.StormForger != nil && s.Application.StormForger.AccessToken != nil {
		return fixRef(s.Application.StormForger.AccessToken.DeepCopy())
	}

	// Check the config file to see if there is something we can use
	if usr, err := user.Current(); err == nil {
		if config, err := toml.LoadFile(filepath.Join(usr.HomeDir, ".stormforger.toml")); err == nil {
			// NOTE: The `[org].jwt` isn't a thing, but we need a way to configure tokens for multiple
			// organizations since service accounts are associated only with a single organization
			for _, key := range []string{org + ".jwt", "jwt"} {
				if v := config.Get(key); v != nil {
					return fixRef(&optimizeappsv1alpha1.StormForgerAccessToken{
						Literal: v.(string),
					})
				}
			}
		}
	}

	return nil
}

func (s *StormForgerSource) stormForgerLatency(lt optimizeappsv1alpha1.LatencyType) string {
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
