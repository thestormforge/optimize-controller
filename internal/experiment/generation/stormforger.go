/*
Copyright 2021 GramLabs, Inc.

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

	"github.com/BurntSushi/toml"
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/scan"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type StormForgerSource struct {
	Scenario    *redskyappsv1alpha1.Scenario
	Application *redskyappsv1alpha1.Application
}

var _ ExperimentSource = &LocustSource{} // Update trial job
var _ MetricSource = &LocustSource{}     // StormForger specific metrics
var _ kio.Reader = &LocustSource{}       // ConfigMap for the test case file

func (s *StormForgerSource) Update(exp *redskyv1beta1.Experiment) error {
	if s.Scenario == nil || s.Application == nil {
		return nil
	}

	env, err := s.stormForgerEnv()
	if err != nil {
		return err
	}

	exp.Spec.TrialTemplate.Spec.JobTemplate = &batchv1beta1.JobTemplateSpec{}
	pod := &exp.Spec.TrialTemplate.Spec.JobTemplate.Spec.Template.Spec
	pod.Containers = []corev1.Container{
		{
			Name:  "stormforger",
			Image: trialJobImage("stormforger"),
			Env:   env,
		},
	}

	// The test case file can be blank, in which case it must be uploaded to StormForger ahead of time
	if s.Scenario.StormForger.TestCaseFile == "" {
		pod.Containers[0].VolumeMounts = append(pod.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "test-case-file",
			ReadOnly:  true,
			MountPath: "/forge-init.d",
		})
		pod.Containers[0].Env = append(pod.Containers[0].Env, corev1.EnvVar{
			Name: "TEST_CASE_FILE",
			// TODO filepath.Base is broken here if TestCaseFile is a URL with a query parameter
			Value: filepath.Join("/forge-init.d", filepath.Base(s.Scenario.StormForger.TestCaseFile)),
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

	return nil
}

func (s *StormForgerSource) Read() ([]*yaml.RNode, error) {
	result := scan.ObjectSlice{}

	// If there is a test case file, create a ConfigMap for it
	if s.Scenario.StormForger.TestCaseFile != "" {
		data, err := loadApplicationData(s.Application, s.Scenario.StormForger.TestCaseFile)
		if err != nil {
			return nil, err
		}

		cm := &corev1.ConfigMap{}
		cm.Name = s.stormForgerConfigMapName()
		// TODO filepath.Base is broken here if TestCaseFile is a URL with a query parameter
		cm.Data = map[string]string{filepath.Base(s.Scenario.StormForger.TestCaseFile): string(data)}
		result = append(result, cm)
	}

	// Include a secret with the access token, if necessary
	if accessToken := s.stormForgerAccessToken(); accessToken != nil {
		switch {
		case accessToken.File != "":
			data, err := loadApplicationData(s.Application, accessToken.File)
			if err != nil {
				return nil, err
			}
			secret := &corev1.Secret{}
			secret.Name = redskyappsv1alpha1.StormForgerAccessTokenSecretName
			secret.Data = map[string][]byte{redskyappsv1alpha1.StormForgerAccessTokenSecretKey: data}
			result = append(result, secret)

		case accessToken.Literal != "":
			secret := &corev1.Secret{}
			secret.Name = redskyappsv1alpha1.StormForgerAccessTokenSecretName
			secret.Data = map[string][]byte{redskyappsv1alpha1.StormForgerAccessTokenSecretKey: []byte(accessToken.Literal)}
			result = append(result, secret)
		}
	}

	return result.Read()
}

func (s *StormForgerSource) Metrics() ([]redskyv1beta1.Metric, error) {
	var result []redskyv1beta1.Metric
	for i := range s.Application.Objectives {
		obj := &s.Application.Objectives[i]
		switch {

		case obj.Latency != nil:
			if l := s.stormForgerLatency(obj.Latency.LatencyType); l != "" {
				query := `scalar(` + l + `{job="trialRun",instance="{{ .Trial.Name }}"})`
				result = append(result, newObjectiveMetric(obj, query))
			}

		case obj.ErrorRate != nil:
			if obj.ErrorRate.ErrorRateType == redskyappsv1alpha1.ErrorRateRequests {
				query := `scalar(error_ratio{job="trialRun",instance="{{ .Trial.Name }}"})`
				result = append(result, newObjectiveMetric(obj, query))
			}

		}
	}
	return result, nil
}

func (s *StormForgerSource) stormForgerConfigMapName() string {
	return fmt.Sprintf("%s-test-case-file", s.Scenario.Name)
}

func (s *StormForgerSource) stormForgerEnv() ([]corev1.EnvVar, error) {
	testCase := s.Scenario.StormForger.TestCase
	if testCase == "" {
		if s.Application.StormForger == nil || s.Application.StormForger.Organization == "" {
			return nil, fmt.Errorf("missing StormForger organization")
		}
		testCase = fmt.Sprintf("%s/%s-%s", s.Application.StormForger.Organization, s.Application.Name, s.Scenario.Name)
	} else if !strings.Contains(testCase, "/") && s.Application.StormForger != nil && s.Application.StormForger.Organization != "" {
		testCase = s.Application.StormForger.Organization + "/" + testCase
	}

	var accessToken *corev1.SecretKeySelector
	if s.Application.StormForger != nil && s.Application.StormForger.AccessToken != nil {
		accessToken = s.Application.StormForger.AccessToken.SecretKeyRef
	}
	if accessToken == nil {
		accessToken = &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: redskyappsv1alpha1.StormForgerAccessTokenSecretName,
			},
			Key: redskyappsv1alpha1.StormForgerAccessTokenSecretKey,
		}
	}

	return []corev1.EnvVar{
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
			Value: testCase,
		},
		{
			Name: "STORMFORGER_JWT",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: accessToken,
			},
		},
	}, nil
}

func (s *StormForgerSource) stormForgerLatency(lt redskyappsv1alpha1.LatencyType) string {
	switch redskyappsv1alpha1.FixLatency(lt) {
	case redskyappsv1alpha1.LatencyMinimum:
		return "min"
	case redskyappsv1alpha1.LatencyMaximum:
		return "max"
	case redskyappsv1alpha1.LatencyMean:
		return "mean"
	case redskyappsv1alpha1.LatencyPercentile50:
		return "median"
	case redskyappsv1alpha1.LatencyPercentile95:
		return "percentile_95"
	case redskyappsv1alpha1.LatencyPercentile99:
		return "percentile_99"
	default:
		return ""
	}
}

func (s *StormForgerSource) stormForgerAccessToken() *redskyappsv1alpha1.StormForgerAccessToken {
	if s.Application.StormForger != nil && s.Application.StormForger.AccessToken != nil {
		return s.Application.StormForger.AccessToken
	}

	if usr, err := user.Current(); err == nil {
		cfg := struct {
			JWT string
		}{}
		if _, err := toml.DecodeFile(filepath.Join(usr.HomeDir, ".stormforger.toml"), &cfg); err == nil {
			return &redskyappsv1alpha1.StormForgerAccessToken{
				Literal: cfg.JWT,
			}
		}
	}

	return nil
}
