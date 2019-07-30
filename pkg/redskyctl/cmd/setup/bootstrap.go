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
package setup

import (
	"fmt"
	"io"
	"strings"

	"github.com/redskyops/k8s-experiment/pkg/api"
	"github.com/redskyops/k8s-experiment/pkg/controller/trial"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	clientbatchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	clientrbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"sigs.k8s.io/yaml"
)

// The bootstrap configuration
type BootstrapConfig struct {
	Namespace          corev1.Namespace
	ClusterRole        rbacv1.ClusterRole
	ClusterRoleBinding rbacv1.ClusterRoleBinding
	Role               rbacv1.Role
	RoleBinding        rbacv1.RoleBinding
	Secret             corev1.Secret
	Job                batchv1.Job

	// Keep an instance of all of the clients we will need for manipulating these objects

	podsClient                clientcorev1.PodInterface
	namespacesClient          clientcorev1.NamespaceInterface
	clusterRolesClient        clientrbacv1.ClusterRoleInterface
	clusterRoleBindingsClient clientrbacv1.ClusterRoleBindingInterface
	rolesClient               clientrbacv1.RoleInterface
	roleBindingsClient        clientrbacv1.RoleBindingInterface
	secretsClient             clientcorev1.SecretInterface
	jobsClient                clientbatchv1.JobInterface
}

// Deletes the bootstrap configuration from the cluster, ignoring all errors
func deleteFromCluster(b *BootstrapConfig) {
	pp := metav1.DeletePropagationForeground
	_ = b.clusterRolesClient.Delete(b.ClusterRole.Name, nil)
	_ = b.clusterRoleBindingsClient.Delete(b.ClusterRoleBinding.Name, nil)
	_ = b.rolesClient.Delete(b.Role.Name, nil)
	_ = b.roleBindingsClient.Delete(b.RoleBinding.Name, nil)
	_ = b.secretsClient.Delete(b.Secret.Name, nil)
	_ = b.jobsClient.Delete(b.Job.Name, &metav1.DeleteOptions{PropagationPolicy: &pp})
}

// Create the bootstrap configuration in the cluster, stopping on the first error
func createInCluster(b *BootstrapConfig) (watch.Interface, error) {
	w, err := b.podsClient.Watch(metav1.ListOptions{LabelSelector: "job-name = " + b.Job.Name})
	if err != nil {
		return nil, err
	}
	if _, err = b.clusterRolesClient.Create(&b.ClusterRole); err != nil {
		return w, err
	}
	if _, err = b.clusterRoleBindingsClient.Create(&b.ClusterRoleBinding); err != nil {
		return w, err
	}
	if _, err = b.rolesClient.Create(&b.Role); err != nil {
		return w, err
	}
	if _, err = b.roleBindingsClient.Create(&b.RoleBinding); err != nil {
		return w, err
	}
	if _, err = b.secretsClient.Create(&b.Secret); err != nil {
		return w, err
	}
	if _, err := b.jobsClient.Create(&b.Job); err != nil {
		return w, err
	}
	return w, nil
}

// Marshal a bootstrap configuration as a YAML stream
func (b *BootstrapConfig) Marshal(w io.Writer) error {
	// Create a scheme with groups we use so type information is generated properly
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	// Collect all of the objects into an array of runtime objects
	var objs []runtime.Object
	objs = append(objs,
		&b.Namespace,
		&b.ClusterRole,
		&b.ClusterRoleBinding,
		&b.Role,
		&b.RoleBinding,
		&b.Secret,
		&b.Job)

	for i := range objs {
		// YAML stream delimiter
		if i > 0 {
			if _, err := w.Write([]byte("---\n")); err != nil {
				return err
			}
		}

		// Use the scheme to convert into a map that contains type information
		u := &unstructured.Unstructured{}
		err := scheme.Convert(objs[i], u, runtime.InternalGroupVersioner)
		if err != nil {
			return err
		}

		// Marshal individual objects as YAML and write the result
		if b, err := yaml.Marshal(u); err != nil {
			return err
		} else if _, err := w.Write(b); err != nil {
			return err
		}
	}

	return nil
}

// NewBootstrapInitConfig creates a complete bootstrap configuration from the supplied values
func NewBootstrapInitConfig(o *SetupOptions, clientConfig *api.Config) (*BootstrapConfig, error) {
	namespace, name := o.namespace, o.name

	// We need a []byte representation of the client configuration for the secret
	secretData, err := yaml.Marshal(clientConfig)
	if err != nil {
		return nil, err
	}

	// Note that we cannot scope "create" roles to a particular resource name

	b := &BootstrapConfig{
		// This is the namespace ultimately used by the product
		Namespace: corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
				Annotations: map[string]string{
					// This is just an attempt to prevent a warning about only using create/apply
					"kubectl.kubernetes.io/last-applied-configuration": fmt.Sprintf(`{"apiVersion":"v1","kind":"Namespace","metadata":{"annotations":{},"labels":{},"name":"%s"}}`, namespace),
				},
			},
		},

		// Bootstrap cluster role bound to the default service account of the namespace
		ClusterRole: rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{rbacv1.VerbAll},
					APIGroups: []string{rbacv1.APIGroupAll},
					Resources: []string{rbacv1.ResourceAll},
				},
			},
		},
		ClusterRoleBinding: rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Namespace: namespace,
					Name:      "default",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     name,
			},
		},

		// Bootstrap role bound to the default service account of the namespace
		Role: rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{rbacv1.VerbAll},
					APIGroups: []string{rbacv1.APIGroupAll},
					Resources: []string{rbacv1.ResourceAll},
				},
			},
		},
		RoleBinding: rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Namespace: namespace,
					Name:      "default",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     name,
			},
		},

		// This bootstrap secret is used as input to a kustomization during installation
		Secret: corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Data:       map[string][]byte{"client.yaml": secretData},
		},

		// The job does a `kubectl apply` to the manifests of the product
		Job: batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{"app": "redsky", "role": "install"},
			},
			Spec: batchv1.JobSpec{
				BackoffLimit:            new(int32),
				TTLSecondsAfterFinished: new(int32),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						Containers: []corev1.Container{
							{
								Name:            "setuptools-install",
								Image:           trial.DefaultImage,
								ImagePullPolicy: corev1.PullAlways,
								Args:            []string{"install"},
								Env: []corev1.EnvVar{
									{
										Name: "NAMESPACE",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												FieldPath: "metadata.namespace",
											},
										},
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "client-config",
										MountPath: "/redskyops/install/client.yaml",
										SubPath:   "client.yaml",
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "client-config",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: name,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Development settings enabled when the default image name did not come from the public repository
	if !strings.Contains(trial.DefaultImage, "/") {
		b.Job.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullIfNotPresent
		*b.Job.Spec.TTLSecondsAfterFinished = 120
	}

	// Create, but do not execute the job
	if o.Bootstrap && !o.DryRun {
		b.Job.Spec.Parallelism = new(int32)
	}

	// Request generation of manifests only
	if o.DryRun && !o.Bootstrap {
		b.Job.Spec.Template.Spec.Containers[0].Args = append(b.Job.Spec.Template.Spec.Containers[0].Args, "--dry-run")
	}

	applyClientSet(b, o)
	return b, nil
}

// NewBootstrapResetConfig creates a configuration for performing a reset
func NewBootstrapResetConfig(o *SetupOptions) (*BootstrapConfig, error) {
	// TODO Really there is a bunch of stuff that doesn't need to be in here for reset
	b, err := NewBootstrapInitConfig(o, nil)
	if err != nil {
		return nil, err
	}
	b.Job.Spec.Template.Spec.Containers[0].Name = "setuptools-uninstall"
	b.Job.Spec.Template.Spec.Containers[0].Args[0] = "uninstall"
	return b, nil
}

func applyClientSet(b *BootstrapConfig, o *SetupOptions) {
	b.podsClient = o.ClientSet.CoreV1().Pods(o.namespace)
	b.namespacesClient = o.ClientSet.CoreV1().Namespaces()
	b.clusterRolesClient = o.ClientSet.RbacV1().ClusterRoles()
	b.clusterRoleBindingsClient = o.ClientSet.RbacV1().ClusterRoleBindings()
	b.rolesClient = o.ClientSet.RbacV1().Roles(o.namespace)
	b.roleBindingsClient = o.ClientSet.RbacV1().RoleBindings(o.namespace)
	b.secretsClient = o.ClientSet.CoreV1().Secrets(o.namespace)
	b.jobsClient = o.ClientSet.BatchV1().Jobs(o.namespace)
}
