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

package prometheus

import (
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/application/experiment/k8s"
	"github.com/thestormforge/optimize-controller/internal/setup"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// AddSetupTask adds Prometheus configuration to the supplied list.
func AddSetupTask(list *corev1.List) {
	// Return if we see the Prometheus setup task
	exp := k8s.FindOrAddExperiment(list)
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
	k8s.EnsureSetupServiceAccount(list)

	// Append the cluster role and binding for Prometheus
	list.Items = append(list.Items,
		runtime.RawExtension{Object: &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "redsky-prometheus",
			},
			Rules: []rbacv1.PolicyRule{
				// Required to manage the Prometheus resources in the setup task
				{
					Verbs:     []string{"get", "create", "delete"},
					APIGroups: []string{"rbac.authorization.k8s.io"},
					Resources: []string{"clusterroles", "clusterrolebindings"},
				},
				{
					Verbs:     []string{"get", "create", "delete"},
					APIGroups: []string{""},
					Resources: []string{"serviceaccounts", "services", "configmaps"},
				},
				{
					Verbs:     []string{"get", "create", "delete", "list", "watch"},
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
