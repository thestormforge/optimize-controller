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
	redskyappsv1alpha1 "github.com/redskyops/redskyops-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	meta2 "github.com/redskyops/redskyops-controller/internal/meta"
	"github.com/redskyops/redskyops-controller/internal/setup"
	"github.com/redskyops/redskyops-controller/pkg/application"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/kustomize/api/filesys"
)

// Generator generates an application experiment.
type Generator struct {
	// The definition of the application to generate an experiment for.
	Application redskyappsv1alpha1.Application
	// ContainerResourcesSelector are the selectors for determining what application resources to scan for resources lists.
	ContainerResourcesSelector []ContainerResourcesSelector

	// File system to use when looking for resources, generally a pass through to the OS file system.
	fs filesys.FileSystem
}

// Create a new generator.
func NewGenerator(fs filesys.FileSystem) *Generator {
	return &Generator{
		fs: fs,
	}
}

// Generate scans the application and produces a list of Kubernetes objects representing an the experiment
func (g *Generator) Generate() (*corev1.List, error) {
	// Load all of the application resources
	arm, err := application.LoadResources(&g.Application, g.fs)
	if err != nil {
		return nil, err
	}

	// Start with an empty list
	list := &corev1.List{}

	// Scan the application resources for requests/limits and add the parameters and patches necessary
	if err := g.scanForContainerResources(arm, list); err != nil {
		return nil, err
	}

	// Add metrics based on the objectives of the application
	if err := g.addObjectives(list); err != nil {
		return nil, err
	}

	// Add a trial job based on the scenario of the application
	if err := g.addScenario(arm, list); err != nil {
		return nil, err
	}

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
			acc.SetName(application.ExperimentName(&g.Application))

			// Add the application label to the templates
			meta2.AddLabel(&obj.Spec.TrialTemplate, redskyappsv1alpha1.LabelApplication, g.Application.Name)
			if obj.Spec.TrialTemplate.Spec.JobTemplate != nil {
				meta2.AddLabel(obj.Spec.TrialTemplate.Spec.JobTemplate, redskyappsv1alpha1.LabelApplication, g.Application.Name)
				meta2.AddLabel(&obj.Spec.TrialTemplate.Spec.JobTemplate.Spec.Template, redskyappsv1alpha1.LabelApplication, g.Application.Name)
			}

			obj.TypeMeta.SetGroupVersionKind(redskyv1beta1.GroupVersion.WithKind("Experiment"))

		case *rbacv1.ClusterRoleBinding:
			for i := range obj.Subjects {
				if obj.Subjects[i].Namespace == "" {
					obj.Subjects[i].Namespace = g.Application.Namespace
				}
			}
		}
	}

	return list, nil
}

// findOrAddExperiment returns the experiment from the supplied list, creating it if it does not exist.
func findOrAddExperiment(list *corev1.List) *redskyv1beta1.Experiment {
	var exp *redskyv1beta1.Experiment
	for i := range list.Items {
		if p, ok := list.Items[i].Object.(*redskyv1beta1.Experiment); ok {
			exp = p
			break
		}
	}
	if exp == nil {
		exp = &redskyv1beta1.Experiment{}
		list.Items = append(list.Items, runtime.RawExtension{Object: exp})
	}
	return exp
}

// ensureSetupServiceAccount ensures that we are using an explicit service account for setup tasks.
func ensureSetupServiceAccount(list *corev1.List) {
	// Return if we see an explicit service account name
	exp := findOrAddExperiment(list)
	saName := &exp.Spec.TrialTemplate.Spec.SetupServiceAccountName
	if *saName != "" {
		return
	}
	*saName = "redsky-setup"

	// Add the actual service account to the list
	list.Items = append(list.Items,
		runtime.RawExtension{Object: &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: *saName,
			},
		}},
	)
}

// ensurePrometheus adds Prometheus configuration to the supplied list.
func ensurePrometheus(list *corev1.List) {
	// Return if we see the Prometheus setup task
	exp := findOrAddExperiment(list)
	trialSpec := &exp.Spec.TrialTemplate.Spec
	for _, st := range trialSpec.SetupTasks {
		if setup.IsPrometheusSetupTask(&st) {
			return
		}
	}

	// Add the missing setup task
	trialSpec.SetupTasks = append(trialSpec.SetupTasks, redskyv1beta1.SetupTask{
		Name: "monitoring",
		Args: []string{"prometheus", "$(MODE)"},
	})

	// Ensure there is an explicit service account configured
	ensureSetupServiceAccount(list)

	// Append the cluster role and binding for Prometheus
	list.Items = append(list.Items,
		runtime.RawExtension{Object: &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "redsky-prometheus",
			},
			Rules: []rbacv1.PolicyRule{
				// Required to manage the Prometheus resources in the setup task
				// TODO It's unclear why this isn't just create/delete on all six types
				{
					Verbs:     []string{"get", "create", "delete"},
					APIGroups: []string{"rbac.authorization.k8s.io"},
					Resources: []string{"clusterroles", "clusterrolebindings"},
				},
				{
					Verbs:     []string{"get", "create", "update"},
					APIGroups: []string{""},
					Resources: []string{"serviceaccounts", "services", "configmaps"},
				},
				{
					Verbs:     []string{"get", "create", "delete", "patch"},
					APIGroups: []string{"apps"},
					Resources: []string{"deployments"},
				},

				// Permissions we need to delegate to Prometheus runtime (prometheus-server-rbac.yaml)
				{
					Verbs:     []string{"list", "watch", "get"},
					APIGroups: []string{""},
					Resources: []string{"nodes", "nodes/metrics", "nodes/proxy", "services"},
				},
				{
					Verbs:     []string{"list", "watch"},
					APIGroups: []string{""},
					Resources: []string{"pods"},
				},
			},
		}},
		runtime.RawExtension{Object: &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "redsky-setup-prometheus",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "redsky-prometheus",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "ServiceAccount",
					Name: exp.Spec.TrialTemplate.Spec.SetupServiceAccountName,
				},
			},
		}},
	)

}
