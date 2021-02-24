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
	"fmt"

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	meta2 "github.com/thestormforge/optimize-controller/internal/meta"
	"github.com/thestormforge/optimize-controller/pkg/application"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/hasher"
)

// Generator generates an application experiment.
type Generator struct {
	// The definition of the application to generate an experiment for.
	Application redskyappsv1alpha1.Application
	// ContainerResourcesSelectors are the selectors for determining what application resources to scan for resources lists.
	ContainerResourcesSelectors []ContainerResourcesSelector
	// ReplicaSelectors are the selectors for determining what application resources to scan for desired replica counts.
	ReplicaSelectors []ReplicaSelector
	// IncludeApplicationResources is a flag indicating that the application resources should be included in the output.
	IncludeApplicationResources bool
	// File system to use when looking for resources, generally a pass through to the OS file system.
	fs filesys.FileSystem
}

// Create a new generator.
func NewGenerator(fs filesys.FileSystem) *Generator {
	return &Generator{
		fs: fs,
	}
}

// SetDefaultSelectors adds the default selectors to the generator.
func (g *Generator) SetDefaultSelectors() {
	// TODO This is kind of a hack: we are just adding labels (if present) to the default selectors
	g.ContainerResourcesSelectors = DefaultContainerResourcesSelectors()
	if g.Application.Parameters != nil && g.Application.Parameters.ContainerResources != nil {
		for i := range g.ContainerResourcesSelectors {
			g.ContainerResourcesSelectors[i].LabelSelector = g.Application.Parameters.ContainerResources.LabelSelector
		}
	}

	if g.Application.Parameters != nil && g.Application.Parameters.Replicas != nil {
		g.ReplicaSelectors = DefaultReplicaSelectors()
		for i := range g.ReplicaSelectors {
			g.ReplicaSelectors[i].LabelSelector = g.Application.Parameters.Replicas.LabelSelector
		}
	}
}

// Generate scans the application and produces a list of Kubernetes objects representing an the experiment
func (g *Generator) Generate() (*corev1.List, error) {
	// Load all of the application resources
	arm, err := application.LoadResources(&g.Application, g.fs)
	if err != nil {
		return nil, err
	}

	// Start with empty lists
	list := &corev1.List{}
	ars := make([]*applicationResource, 0, arm.Size())

	// Scan the application resources for requests/limits and add the parameters and patches necessary
	ars, err = g.scanForContainerResources(ars, arm)
	if err != nil {
		return nil, err
	}

	// Scan the application resources for replicas and add the parameters and patches necessary
	ars, err = g.scanForReplicas(ars, arm)
	if err != nil {
		return nil, err
	}

	// Add parameters and patches discovered from the application
	if err := patchExperiment(ars, list); err != nil {
		return nil, err
	}

	// Add a trial job based on the scenario of the application
	// NOTE: Some objectives may have scenario specific implementations which will also be implemented
	if err := g.addScenario(arm, list); err != nil {
		return nil, err
	}

	// Add metrics based on the objectives of the application
	if err := g.addObjectives(list); err != nil {
		return nil, err
	}

	// We need to ensure cluster resources have a unique name based on the experiment to avoid conflict
	experimentName := application.ExperimentName(&g.Application)
	clusterRoleNameSuffix := fmt.Sprintf("-%s", hasher.Hash(experimentName)[0:6])

	// Update the metadata of the generated objects
	for i := range list.Items {
		// Get a generic accessor for the list item
		acc, err := meta.Accessor(list.Items[i].Object)
		if err != nil {
			return nil, err
		}

		// Label all objects with the application name
		meta2.AddLabel(acc, redskyappsv1alpha1.LabelApplication, g.Application.Name)

		switch obj := list.Items[i].Object.(type) {

		case *corev1.ServiceAccount, *corev1.ConfigMap, *corev1.Secret:
			acc.SetNamespace(g.Application.Namespace)

		case *redskyv1beta1.Experiment:
			acc.SetNamespace(g.Application.Namespace)
			acc.SetName(experimentName)
			labelExperiment(&g.Application, obj)

		case *rbacv1.ClusterRole:
			obj.Name += clusterRoleNameSuffix

		case *rbacv1.ClusterRoleBinding:
			obj.Name += clusterRoleNameSuffix
			obj.RoleRef.Name += clusterRoleNameSuffix
			for i := range obj.Subjects {
				if obj.Subjects[i].Namespace == "" {
					obj.Subjects[i].Namespace = g.Application.Namespace
				}
			}
		}
	}

	// Verify all of the objectives were implemented
	for i := range g.Application.Objectives {
		if !g.Application.Objectives[i].Implemented {
			return nil, fmt.Errorf("generated experiment cannot optimize objective: %s", g.Application.Objectives[i].Name)
		}
	}

	// If requested, append the actual application resources to the output
	if g.IncludeApplicationResources {
		for _, r := range arm.Resources() {
			list.Items = append(list.Items, runtime.RawExtension{Object: &unstructured.Unstructured{Object: r.Map()}})
		}
	}

	return list, nil
}

// labelExperiment adds application labels to the experiment
func labelExperiment(app *redskyappsv1alpha1.Application, exp *redskyv1beta1.Experiment) {
	// This function adds labels to the experiment, the trial template and (if present)
	// the job template (including the job's pod template).
	addExpLabel := func(label, value string) {
		meta2.AddLabel(exp, label, value)
		meta2.AddLabel(&exp.Spec.TrialTemplate, label, value)
		if exp.Spec.TrialTemplate.Spec.JobTemplate != nil {
			meta2.AddLabel(exp.Spec.TrialTemplate.Spec.JobTemplate, label, value)
			meta2.AddLabel(&exp.Spec.TrialTemplate.Spec.JobTemplate.Spec.Template, label, value)
		}
	}

	if app.Name != "" {
		addExpLabel(redskyappsv1alpha1.LabelApplication, app.Name)
	}

	if len(app.Scenarios) == 1 {
		addExpLabel(redskyappsv1alpha1.LabelScenario, app.Scenarios[0].Name)
	}
}
