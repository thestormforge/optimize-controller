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
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/internal/sfio"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/kyaml/kio"
)

type PrometheusMetricsSource struct {
	Goal *optimizeappsv1alpha1.Goal
}

var _ MetricSource = &PrometheusMetricsSource{}

func (s *PrometheusMetricsSource) Metrics() ([]optimizev1beta2.Metric, error) {
	var result []optimizev1beta2.Metric
	if s.Goal == nil || s.Goal.Implemented {
		return result, nil
	}

	m := newGoalMetric(s.Goal, s.Goal.Prometheus.Query)
	m.URL = s.Goal.Prometheus.URL
	m.Minimize = !s.Goal.Prometheus.Maximize
	result = append(result, m)

	return result, nil
}

type BuiltInPrometheus struct {
	SetupTaskName          string
	ClusterRoleName        string
	ServiceAccountName     string
	ClusterRoleBindingName string

	sfio.ObjectSlice
}

var _ ExperimentSource = &BuiltInPrometheus{} // Service Account name and Setup Task
var _ kio.Reader = &BuiltInPrometheus{}       // RBAC

func (p *BuiltInPrometheus) Update(exp *optimizev1beta2.Experiment) error {
	// Detect if we need built-in Prometheus by checking the generated metrics
	var needsPrometheus bool
	for _, m := range exp.Spec.Metrics {
		if m.Type == optimizev1beta2.MetricPrometheus && m.URL == "" {
			needsPrometheus = true
			break
		}
	}

	if !needsPrometheus {
		return nil
	}

	exp.Spec.TrialTemplate.Spec.SetupServiceAccountName = p.ServiceAccountName
	exp.Spec.TrialTemplate.Spec.SetupTasks = append(exp.Spec.TrialTemplate.Spec.SetupTasks,
		optimizev1beta2.SetupTask{
			Name: p.SetupTaskName,
			Args: []string{"prometheus", "$(MODE)"},
		})

	p.ObjectSlice = append(p.ObjectSlice,
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: p.ServiceAccountName,
			},
		},

		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: p.ClusterRoleName,
			},
			Rules: []rbacv1.PolicyRule{
				// Required to manage the Prometheus resources in the setup task
				{
					Verbs:     []string{"get", "create", "delete"},
					APIGroups: []string{rbacv1.GroupName},
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
		},

		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: p.ClusterRoleBindingName,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     p.ClusterRoleName,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "ServiceAccount",
					Name: p.ServiceAccountName,
				},
			},
		},
	)

	return nil
}
