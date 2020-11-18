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

package stormforger

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	redskyappsv1alpha1 "github.com/redskyops/redskyops-controller/api/apps/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/generate/experiment/k8s"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/generate/experiment/prometheus"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/ifc"
	"sigs.k8s.io/kustomize/api/loader"
	"sigs.k8s.io/kustomize/api/resmap"
)

func AddTrialJob(sc *redskyappsv1alpha1.Scenario, app *redskyappsv1alpha1.Application, fs filesys.FileSystem, rm resmap.ResMap, list *corev1.List) error {
	// Fail unless we have the necessary global configuration
	if app.StormForger == nil {
		return fmt.Errorf("the StormForger global configuration is required when using a StormForger scenario")
	}

	// Find the experiment and initialize a new job
	exp := k8s.FindOrAddExperiment(list)
	exp.Spec.TrialTemplate.Spec.JobTemplate = &batchv1beta1.JobTemplateSpec{}

	// The StormForger image we are about to use requires Prometheus for the Push Gateway
	prometheus.AddSetupTask(list)

	// Add metrics for the scenario specific objectives
	if err := addStormForgerObjectives(app, list); err != nil {
		return err
	}

	// Create a loader so we can read "files"
	ldr, err := loader.NewLoader(loader.RestrictionNone, ".", fs)
	if err != nil {
		return err
	}

	// Add a new container to run the StormForger integration
	pod := &exp.Spec.TrialTemplate.Spec.JobTemplate.Spec.Template.Spec
	pod.Containers = []corev1.Container{
		{
			Name:  "stormforger",
			Image: stormForgerImage(),
			Env: []corev1.EnvVar{
				{
					Name: "TITLE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
			},
		},
	}

	// Add the test case name
	if testCase, err := testCase(sc, app); err != nil {
		return err
	} else if testCase != nil {
		pod.Containers[0].Env = append(pod.Containers[0].Env, *testCase)
	}

	// Add a ConfigMap with the test case script
	if testCaseFile, testCaseVolumeMount, testCaseVolume, err := ensureStormForgerTestCaseFile(sc.StormForger, ldr, list); err != nil {
		return err
	} else if testCaseFile != nil && testCaseVolumeMount != nil && testCaseVolume != nil {
		pod.Containers[0].Env = append(pod.Containers[0].Env, *testCaseFile)
		pod.Containers[0].VolumeMounts = append(pod.Containers[0].VolumeMounts, *testCaseVolumeMount)
		pod.Volumes = append(pod.Volumes, *testCaseVolume)
	}

	// Scan the application to determine the ingress point
	if target, err := scanForIngress(rm, app.Ingress); err != nil {
		return err
	} else if target != nil {
		pod.Containers[0].Env = append(pod.Containers[0].Env, *target)
	}

	// Add a secret with the StormForger credentials
	if stormForgerJWT, err := ensureStormForgerSecret(app.StormForger.AccessToken, ldr, list); err != nil {
		return err
	} else if stormForgerJWT != nil {
		pod.Containers[0].Env = append(pod.Containers[0].Env, *stormForgerJWT)
	}

	return nil
}

func stormForgerImage() string {
	// Allow the image name to be overridden using environment variables, primarily for development work
	imageName := os.Getenv("TRIAL_JOB_IMAGE_REPOSITORY")
	if imageName == "" {
		imageName = "redskyops/trial-jobs"
	}
	imageTag := os.Getenv("TRIAL_JOB_IMAGE_TAG")
	if imageTag == "" {
		imageTag = "0.0.1-stormforger"
	}
	return imageName + ":" + imageTag
}

func testCase(sc *redskyappsv1alpha1.Scenario, app *redskyappsv1alpha1.Application) (*corev1.EnvVar, error) {
	testCase := &corev1.EnvVar{
		Name:  "TEST_CASE",
		Value: sc.StormForger.TestCase,
	}

	// Compute a default test case name
	if testCase.Value == "" {
		if app.StormForger.Organization == "" {
			return nil, fmt.Errorf("missing StormForger organization")
		}

		testCase.Value = fmt.Sprintf("%s/%s-%s", app.StormForger.Organization, app.Name, sc.Name)
	} else if !strings.Contains(testCase.Value, "/") && app.StormForger.Organization != "" {
		testCase.Value = app.StormForger.Organization + "/" + testCase.Value
	}

	return testCase, nil
}

func ensureStormForgerTestCaseFile(s *redskyappsv1alpha1.StormForgerScenario, ldr ifc.Loader, list *corev1.List) (*corev1.EnvVar, *corev1.VolumeMount, *corev1.Volume, error) {
	// The test case file can be blank, in which case it must be uploaded to StormForger ahead of time
	if s.TestCaseFile == "" {
		return nil, nil, nil, nil
	}

	// TODO Try to find it first...
	testCaseConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "stormforger-test-case",
		},
		BinaryData: map[string][]byte{},
	}
	list.Items = append(list.Items, runtime.RawExtension{Object: testCaseConfigMap})

	// Load the test case file into the config map
	key := filepath.Base(s.TestCaseFile)
	data, err := ldr.Load(s.TestCaseFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to load StormForger test case file: %w", err)
	}
	testCaseConfigMap.BinaryData[key] = data

	testCaseVolume := &corev1.Volume{
		Name: "stormforger-test-case-file",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: testCaseConfigMap.Name,
				},
			},
		},
	}

	testCaseVolumeMount := &corev1.VolumeMount{
		Name:      testCaseVolume.Name,
		ReadOnly:  true,
		MountPath: "/forge-init.d",
	}

	testCaseFile := &corev1.EnvVar{
		Name:  "TEST_CASE_FILE",
		Value: filepath.Join(testCaseVolumeMount.MountPath, key),
	}

	return testCaseFile, testCaseVolumeMount, testCaseVolume, nil
}

func scanForIngress(rm resmap.ResMap, ingress *redskyappsv1alpha1.Ingress) (*corev1.EnvVar, error) {
	if ingress == nil || ingress.URL == "" {
		return nil, nil
	}

	// This needs to find the ingress.
	// I think there are 3 options:
	// 1. Poke around in the resource map
	// 2. Let the job query the Kube API at runtime
	// 3. Let the controller query the Kube API at runtime

	// For 1, we are unlikely to find much: we might be find some objects that have names that are later bound
	// into to something we can hit from outside the cluster (e.g. a host name rule on an ingress object)
	// but most of those will be ambiguous (or provider specific).

	// For 2, there is an RBAC problem: the job will need additional runtime permissions via it's service account
	// to query the Kube API.

	// For 3, the controller may have similar RBAC issues and is further limited by the fact that it is looking at
	// a generic job/trial template for clues about how to do the ingress lookup (at which time it would modify
	// the container definition for the job's pod, prior to creating the job). We might need some kind of Go template
	// in the environment variable values that the controller would evaluate prior to runtime (come to think of it,
	// that might be a more explicit and obvious solution to the current problem of the parameter names magically
	// showing up in the environment); we would just need something to handle the lookup, e.g. a custom function
	// like `externalIP "myservice"` that evaluates prior to job scheduling.

	return &corev1.EnvVar{
		Name:  "TARGET",
		Value: ingress.URL,
	}, nil
}

func ensureStormForgerSecret(at *redskyappsv1alpha1.StormForgerAccessToken, ldr ifc.Loader, list *corev1.List) (*corev1.EnvVar, error) {
	// Create a new environment variable definition
	stormForgerJWT := &corev1.EnvVar{
		Name:      "STORMFORGER_JWT",
		ValueFrom: &corev1.EnvVarSource{},
	}

	// An explicit secret key reference is assumed to reference an existing secret
	if at != nil && at.SecretKeyRef != nil {
		stormForgerJWT.ValueFrom.SecretKeyRef = at.SecretKeyRef
		return stormForgerJWT, nil
	}

	// Use a constant reference
	stormForgerJWT.ValueFrom.SecretKeyRef = &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: redskyappsv1alpha1.StormForgerAccessTokenSecretName,
		},
		Key: redskyappsv1alpha1.StormForgerAccessTokenSecretKey,
	}

	// If we find the secret in the list, we are done
	var secret *corev1.Secret
	for i := range list.Items {
		if s, ok := list.Items[i].Object.(*corev1.Secret); ok {
			if s.Name == stormForgerJWT.ValueFrom.SecretKeyRef.Name {
				secret = s

				// If the key is missing we still have work to do
				if _, ok := s.Data[stormForgerJWT.ValueFrom.SecretKeyRef.Key]; !ok {
					break
				}
				return stormForgerJWT, nil
			}
		}
	}

	// Create a new secret if one does not exist
	if secret == nil {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: stormForgerJWT.ValueFrom.SecretKeyRef.Name,
			},
			Type: corev1.SecretTypeOpaque,
		}
		list.Items = append(list.Items, runtime.RawExtension{Object: secret})
	}

	switch {
	case at == nil:
		cfg := struct {
			JWT string
		}{}
		if usr, err := user.Current(); err == nil {
			_, _ = toml.DecodeFile(filepath.Join(usr.HomeDir, ".stormforger.toml"), &cfg)
		}

		secret.StringData = map[string]string{stormForgerJWT.ValueFrom.SecretKeyRef.Key: cfg.JWT}

	case at.Literal != "":
		secret.StringData = map[string]string{stormForgerJWT.ValueFrom.SecretKeyRef.Key: at.Literal}

	case at.File != "":
		data, err := ldr.Load(at.File)
		if err != nil {
			return nil, fmt.Errorf("unable to load StormForger access token: %w", err)
		}

		secret.Data = map[string][]byte{stormForgerJWT.ValueFrom.SecretKeyRef.Key: data}

	default:
		secret.Data = map[string][]byte{stormForgerJWT.ValueFrom.SecretKeyRef.Key: nil}
	}

	return stormForgerJWT, nil
}
