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

package locust

import (
	"fmt"

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/generate/experiment/k8s"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/generate/experiment/prometheus"
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
	// Find the experiment and initialize a new job
	exp := k8s.FindOrAddExperiment(list)
	exp.Spec.TrialTemplate.Spec.JobTemplate = &batchv1beta1.JobTemplateSpec{}

	// The Locust image we are about to use requires Prometheus for the Pushgateway
	prometheus.AddSetupTask(list)

	// Add metrics for the scenario specific objectives
	if err := addLocustObjectives(app, list); err != nil {
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
			Name:  "locust",
			Image: k8s.TrialJobImage("locust"),
			Env:   []corev1.EnvVar{},
		},
	}

	// Add the configuration environment variables
	if target, err := host(rm, app.Ingress); err != nil {
		return err
	} else if target != nil {
		pod.Containers[0].Env = append(pod.Containers[0].Env, *target)
	}
	if users := users(sc.Locust); users != nil {
		pod.Containers[0].Env = append(pod.Containers[0].Env, *users)
	}
	if spawnRate := spawnRate(sc.Locust); spawnRate != nil {
		pod.Containers[0].Env = append(pod.Containers[0].Env, *spawnRate)
	}
	if runTime := runTime(sc.Locust); runTime != nil {
		pod.Containers[0].Env = append(pod.Containers[0].Env, *runTime)
		exp.Spec.TrialTemplate.Spec.ApproximateRuntime = &metav1.Duration{Duration: *sc.Locust.RunTime}
	}

	// Add a ConfigMap with the Locust file
	if locustVolumeMount, locustVolume, err := ensureLocustFile(sc, ldr, list); err != nil {
		return err
	} else if locustVolumeMount != nil && locustVolume != nil {
		pod.Containers[0].VolumeMounts = append(pod.Containers[0].VolumeMounts, *locustVolumeMount)
		pod.Volumes = append(pod.Volumes, *locustVolume)
	}

	return nil
}

func ensureLocustFile(s *redskyappsv1alpha1.Scenario, ldr ifc.Loader, list *corev1.List) (*corev1.VolumeMount, *corev1.Volume, error) {
	if s.Locust.Locustfile == "" {
		return nil, nil, fmt.Errorf("missing Locust file")
	}

	// TODO Try to find it first...
	locustConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-locustfile", s.Name),
		},
		BinaryData: map[string][]byte{},
	}
	list.Items = append(list.Items, runtime.RawExtension{Object: locustConfigMap})

	// Load the Locust file into the config map
	data, err := ldr.Load(s.Locust.Locustfile)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load Locust file: %w", err)
	}
	locustConfigMap.BinaryData["locustfile.py"] = data

	locustVolume := &corev1.Volume{
		Name: "locustfile",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: locustConfigMap.Name,
				},
			},
		},
	}

	locustVolumeMount := &corev1.VolumeMount{
		Name:      locustVolume.Name,
		ReadOnly:  true,
		MountPath: "/mnt/locust",
	}

	return locustVolumeMount, locustVolume, nil
}

func host(rm resmap.ResMap, ingress *redskyappsv1alpha1.Ingress) (*corev1.EnvVar, error) {
	host, err := k8s.ScanForIngress(rm, ingress, false)
	if err != nil {
		return nil, err
	}

	if host == "" {
		return nil, fmt.Errorf("ingress must be configured when using Locust scenarios")
	}

	return &corev1.EnvVar{
		Name:  "HOST",
		Value: host,
	}, nil
}

func users(s *redskyappsv1alpha1.LocustScenario) *corev1.EnvVar {
	if s.Users == nil {
		return nil
	}
	return &corev1.EnvVar{
		Name:  "NUM_USERS",
		Value: fmt.Sprintf("%d", *s.Users),
	}
}

func spawnRate(s *redskyappsv1alpha1.LocustScenario) *corev1.EnvVar {
	if s.SpawnRate == nil {
		return nil
	}
	return &corev1.EnvVar{
		Name:  "SPAWN_RATE",
		Value: fmt.Sprintf("%d", *s.SpawnRate),
	}
}

func runTime(s *redskyappsv1alpha1.LocustScenario) *corev1.EnvVar {
	if s.RunTime == nil {
		return nil
	}
	return &corev1.EnvVar{
		Name:  "RUN_TIME",
		Value: s.RunTime.String(),
	}
}
