/*
Copyright 2019 GramLabs, Inc.

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
	"context"

	"github.com/redskyops/redskyops-controller/internal/trial"
	redskyv1alpha1 "github.com/redskyops/redskyops-controller/pkg/apis/redsky/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NextTrialNamespace searches for or creates a new namespace to run a new trial in, returning an empty string if no such namespace can be found
func NextTrialNamespace(ctx context.Context, c client.Client, exp *redskyv1alpha1.Experiment, trialList *redskyv1alpha1.TrialList) (string, error) {
	// Determine which namespaces have an active trial
	activeNamespaces := make(map[string]bool, len(trialList.Items))
	activeTrials := int32(0)
	for i := range trialList.Items {
		t := &trialList.Items[i]
		if trial.IsActive(t) {
			activeNamespaces[t.Namespace] = true
			activeTrials++
		}
	}

	// Check the number of desired replicas
	if activeTrials >= exp.Replicas() || exp.Status.ActiveTrials != activeTrials {
		return "", nil
	}

	// Match the potential namespaces
	var selector client.ListOption
	if n := exp.Spec.Template.Namespace; n != "" {
		// If there is an explicit target namespace on the trial template it is the only one we will be allowed to use
		selector = client.MatchingFields{"metadata.name": n}
	} else if exp.Spec.NamespaceSelector == nil && exp.Spec.NamespaceTemplate == nil {
		// If there is no namespace selector/template we can only use the experiment namespace
		selector = client.MatchingFields{"metadata.name": exp.Namespace}
	} else {
		// Match the (possibly nil) namespace selector
		s, err := metav1.LabelSelectorAsSelector(exp.Spec.NamespaceSelector)
		if err != nil {
			return "", err
		}
		selector = client.MatchingLabelsSelector{Selector: s}
	}

	// Find the first available namespace from the list
	namespaceList := &corev1.NamespaceList{}
	if err := c.List(ctx, namespaceList, selector); err != nil {
		return "", err
	}
	for i := range namespaceList.Items {
		if n := namespaceList.Items[i].Name; !activeNamespaces[n] {
			return n, nil
		}
	}

	// If we could not find a namespace, we may be able to create it
	if exp.Spec.NamespaceTemplate != nil {
		return createNamespaceFromTemplate(c, ctx, exp)
	}

	// No namespace is available
	return "", nil
}

func ignorePermissions(err error) error {
	if apierrs.IsUnauthorized(err) {
		return nil
	}
	if apierrs.IsForbidden(err) {
		return nil
	}
	return err
}

func createNamespaceFromTemplate(c client.Client, ctx context.Context, exp *redskyv1alpha1.Experiment) (string, error) {
	// Use the template to populate a new namespace
	n := &corev1.Namespace{}
	exp.Spec.NamespaceTemplate.ObjectMeta.DeepCopyInto(&n.ObjectMeta)
	exp.Spec.NamespaceTemplate.Spec.DeepCopyInto(&n.Spec)
	if n.Name == "" && n.GenerateName == "" {
		n.GenerateName = exp.Name + "-"
	}
	if n.Labels == nil {
		n.Labels = map[string]string{}
	}
	n.Labels[redskyv1alpha1.LabelExperiment] = exp.Name
	n.Labels[redskyv1alpha1.LabelTrialRole] = "trialSetup"

	// TODO We should also record the fact that we created the namespace for possible clean up later

	// NOTE: The ignorePermission call is in different places for the namespace and supporting objects because
	// if the namespace creation fails we cannot continue creating the supporting objects
	if err := c.Create(ctx, n); err != nil {
		// Ignore duplicates, e.g. it is possible that the namespace template has an explicit name
		if apierrs.IsAlreadyExists(err) || ignorePermissions(err) == nil {
			return "", nil
		}
		// TODO Fail the experiment? Set replicas to activeTrials? Just ignore log it and don't do anything?
		return "", err
	}

	// Create the support trial namespace objects
	ts := createTrialNamespace(exp, n.Name)
	if ts.ServiceAccount != nil {
		if err := c.Create(ctx, ts.ServiceAccount); ignorePermissions(err) != nil {
			return "", err
		}
	}
	if ts.Role != nil {
		if err := c.Create(ctx, ts.Role); ignorePermissions(err) != nil {
			return "", err
		}
	}
	for i := range ts.RoleBindings {
		if err := c.Create(ctx, &ts.RoleBindings[i]); ignorePermissions(err) != nil {
			return "", err
		}
	}

	return n.Name, nil
}

// trialNamespace represents the supporting resources for a trial namespace
type trialNamespace struct {
	ServiceAccount *corev1.ServiceAccount
	Role           *rbacv1.Role
	RoleBindings   []rbacv1.RoleBinding
}

func createTrialNamespace(exp *redskyv1alpha1.Experiment, namespace string) *trialNamespace {
	ts := &trialNamespace{}

	// Fill in the details about the service account
	ts.ServiceAccount = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      exp.Spec.Template.Spec.SetupServiceAccountName,
			Namespace: namespace,
		},
	}
	if ts.ServiceAccount.Name == "" {
		ts.ServiceAccount.Name = "default"
	}

	// Add a namespaced role and binding based on the default setup task policy rules
	if len(exp.Spec.Template.Spec.SetupDefaultRules) > 0 {
		ts.Role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "redsky-setup-role",
				Namespace: namespace,
			},
			Rules: exp.Spec.Template.Spec.SetupDefaultRules,
		}

		ts.RoleBindings = append(ts.RoleBindings, rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "redsky-setup-rolebinding",
				Namespace: namespace,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      ts.ServiceAccount.Name,
				Namespace: ts.ServiceAccount.Namespace,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     ts.Role.Name,
			},
		})
	}

	// Add a namespaced role binding to a (presumably existing) cluster role
	if exp.Spec.Template.Spec.SetupDefaultClusterRole != "" {
		ts.RoleBindings = append(ts.RoleBindings, rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "redsky-setup-cluster-rolebinding",
				Namespace: namespace,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      ts.ServiceAccount.Name,
				Namespace: ts.ServiceAccount.Namespace,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     exp.Spec.Template.Spec.SetupDefaultClusterRole,
			},
		})
	}

	// Don't actually return the default service account for creation
	if ts.ServiceAccount.Name == "default" {
		ts.ServiceAccount = nil
	}

	return ts
}
