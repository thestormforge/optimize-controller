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
	"reflect"

	"github.com/redskyops/k8s-experiment/pkg/api"
	"github.com/redskyops/k8s-experiment/pkg/controller/trial"
	"github.com/redskyops/k8s-experiment/pkg/version"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientbatchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	clientrbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"sigs.k8s.io/yaml"
)

// BootstrapConfig holds the objects necessary for bootstrapping a Red Sky Ops Manager
type BootstrapConfig struct {

	// Bootstrap objects required to run the setuptools install job

	Namespace          corev1.Namespace
	ClusterRole        rbacv1.ClusterRole
	ClusterRoleBinding rbacv1.ClusterRoleBinding
	Role               rbacv1.Role
	RoleBinding        rbacv1.RoleBinding
	Secret             corev1.Secret

	// RBAC objects required for runtime

	PatchingClusterRole        rbacv1.ClusterRole
	PatchingClusterRoleBinding rbacv1.ClusterRoleBinding
	DefaultPatchingClusterRole rbacv1.ClusterRole

	// The actual setuptools install job

	Job batchv1.Job

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

// Create the bootstrap configuration in the cluster, stopping on the first error
func createInCluster(b *BootstrapConfig) error {
	var err error
	if _, err = b.namespacesClient.Create(&b.Namespace); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	if _, err = b.clusterRolesClient.Create(&b.ClusterRole); err != nil {
		return err
	}
	if _, err = b.clusterRoleBindingsClient.Create(&b.ClusterRoleBinding); err != nil {
		return err
	}
	if _, err = b.rolesClient.Create(&b.Role); err != nil {
		return err
	}
	if _, err = b.roleBindingsClient.Create(&b.RoleBinding); err != nil {
		return err
	}
	if _, err = b.secretsClient.Create(&b.Secret); err != nil {
		return err
	}
	if _, err := b.clusterRolesClient.Get(b.PatchingClusterRole.Name, metav1.GetOptions{}); err != nil {
		// Only try to create these resources once, then let them persist through updates
		if !errors.IsNotFound(err) {
			return err
		}
		// TODO We should always force update the role and binding to match the bootstrap config
		if _, err = b.clusterRolesClient.Create(&b.PatchingClusterRole); err != nil {
			return err
		}
		if _, err = b.clusterRoleBindingsClient.Create(&b.PatchingClusterRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
		// TODO We should offer an init option to overwrite this to the current defaults
		if _, err = b.clusterRolesClient.Create(&b.DefaultPatchingClusterRole); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	if _, err := b.jobsClient.Create(&b.Job); err != nil {
		return err
	}

	return nil
}

// Deletes the bootstrap configuration from the cluster, ignoring all errors
func deleteFromCluster(b *BootstrapConfig) {
	// DO NOT delete the namespace or patching roles

	_ = b.clusterRolesClient.Delete(b.ClusterRole.Name, nil)
	_ = b.clusterRoleBindingsClient.Delete(b.ClusterRoleBinding.Name, nil)
	_ = b.rolesClient.Delete(b.Role.Name, nil)
	_ = b.roleBindingsClient.Delete(b.RoleBinding.Name, nil)
	_ = b.secretsClient.Delete(b.Secret.Name, nil)

	pp := metav1.DeletePropagationForeground
	_ = b.jobsClient.Delete(b.Job.Name, &metav1.DeleteOptions{PropagationPolicy: &pp})
}

// Deletes the application configuration from the cluster, ignoring all errors
func resetFromCluster(b *BootstrapConfig) {
	_ = b.clusterRolesClient.Delete(b.PatchingClusterRole.Name, nil)
	_ = b.clusterRoleBindingsClient.Delete(b.PatchingClusterRoleBinding.Name, nil)

	// TODO Should we leave this behind? Should we check for updates and only delete it in the default state?
	_ = b.clusterRolesClient.Delete(b.DefaultPatchingClusterRole.Name, nil)
}

// Marshal a bootstrap configuration as a YAML stream
func (b *BootstrapConfig) Marshal(w io.Writer) error {
	// Create a scheme with groups we use so type information is generated properly
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	// Iterate over the fields of the bootstrap configuration
	val := reflect.ValueOf(b).Elem()
	for i := 0; i < val.NumField(); i++ {
		// Only take the fields whose address is a runtime object
		if !val.Field(i).CanAddr() || !val.Field(i).Addr().CanInterface() {
			continue
		}
		f, ok := val.Field(i).Addr().Interface().(runtime.Object)
		if !ok {
			continue
		}

		// YAML stream delimiter
		if i > 0 {
			if _, err := w.Write([]byte("---\n")); err != nil {
				return err
			}
		}

		// Use the scheme to convert into a map that contains type information
		u := &unstructured.Unstructured{}
		err := scheme.Convert(f, u, runtime.InternalGroupVersioner)
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

	// We need to run as a non-root user that has the same UID and GID
	id := int64(1000)
	allowPrivilegeEscalation := false
	runAsNonRoot := true

	// Verbs used by `kubectl apply` during installation
	applyVerbs := []string{"get", "create", "patch", "delete"}

	b := &BootstrapConfig{
		// This is the namespace ultimately used by the product
		Namespace: corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "redskyops",
					"app.kubernetes.io/version":    version.Version,
					"app.kubernetes.io/managed-by": "redskyctl",
				},
				Annotations: map[string]string{
					// This is just an attempt to prevent a warning about only using create/apply
					"kubectl.kubernetes.io/last-applied-configuration": fmt.Sprintf(`{"apiVersion":"v1","kind":"Namespace","metadata":{"annotations":{},"labels":{},"name":"%s"}}`, namespace),
				},
			},
		},

		// The bootstrap cluster role serves two purposes: first we must be able to create cluster scoped objects for
		// the application (namespace and CRD); second, we must create cluster roles for the application which means
		// we would either need escalation privileges or at least as many permissions as we are going to grant to the
		// application roles.
		ClusterRole: rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "redskyops",
					"app.kubernetes.io/version":    version.Version,
					"app.kubernetes.io/component":  "install",
					"app.kubernetes.io/managed-by": "redskyctl",
				},
			},
			Rules: []rbacv1.PolicyRule{
				// Application objects
				{
					APIGroups: []string{""},
					Resources: []string{"namespaces"},
					Verbs:     applyVerbs,
				},
				{
					APIGroups: []string{"apiextensions.k8s.io"},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     applyVerbs,
				},
				{
					APIGroups: []string{"rbac.authorization.k8s.io"},
					Resources: []string{"clusterroles", "clusterrolebindings"},
					Verbs:     applyVerbs,
				},
				// Manager Role (role.yaml)
				{
					APIGroups: []string{""},
					Resources: []string{"namespaces", "pods", "services"},
					Verbs:     []string{"list"},
				},
				{
					APIGroups: []string{"batch", "extensions"},
					Resources: []string{"jobs", "jobs/status"},
					Verbs:     []string{rbacv1.VerbAll},
				},
				{
					APIGroups: []string{"redskyops.dev"},
					Resources: []string{rbacv1.ResourceAll},
					Verbs:     []string{rbacv1.VerbAll},
				},
				// Authentication Proxy Role (auth_proxy_role.yaml)
				{
					APIGroups: []string{"authentication.k8s.io"},
					Resources: []string{"tokenreviews"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{"authorization.k8s.io"},
					Resources: []string{"subjectaccessreviews"},
					Verbs:     []string{"create"},
				},
				// Patching Roles (patching_role.yaml, rbac_footer.txt)
				// ...added programmatically below
			},
		},
		ClusterRoleBinding: rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "redskyops",
					"app.kubernetes.io/version":    version.Version,
					"app.kubernetes.io/component":  "install",
					"app.kubernetes.io/managed-by": "redskyctl",
				},
			},
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

		// The bootstrap role is used by the setuptools container to actually install the application
		Role: rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "redskyops",
					"app.kubernetes.io/version":    version.Version,
					"app.kubernetes.io/component":  "install",
					"app.kubernetes.io/managed-by": "redskyctl",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"secrets", "services"},
					Verbs:     applyVerbs,
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"deployments"},
					Verbs:     applyVerbs,
				},
			},
		},
		RoleBinding: rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "redskyops",
					"app.kubernetes.io/version":    version.Version,
					"app.kubernetes.io/component":  "install",
					"app.kubernetes.io/managed-by": "redskyctl",
				},
			},
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
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "redskyops",
					"app.kubernetes.io/version":    version.Version,
					"app.kubernetes.io/component":  "install",
					"app.kubernetes.io/managed-by": "redskyctl",
				},
			},
			Data: map[string][]byte{"client.yaml": secretData},
		},

		// The job does a `kubectl apply` to the manifests of the product
		Job: batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "redskyops",
					"app.kubernetes.io/version":    version.Version,
					"app.kubernetes.io/component":  "install",
					"app.kubernetes.io/managed-by": "redskyctl",
				},
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
								Image:           trial.Image,
								ImagePullPolicy: corev1.PullPolicy(trial.ImagePullPolicy),
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
								SecurityContext: &corev1.SecurityContext{
									RunAsUser:                &id,
									RunAsGroup:               &id,
									AllowPrivilegeEscalation: &allowPrivilegeEscalation,
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "client-config",
										MountPath: "/workspace/install/client.yaml",
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
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: &runAsNonRoot,
						},
					},
				},
			},
		},

		// IMPORTANT: Changes to the patching role and the default need to be reflected in the static configuration
		// maintained for the Helm chart (patching_role.yaml and rbac_footer.txt)

		// This role allows individual deployments more flexibility in defining what can be patched by manager
		PatchingClusterRole: rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "redsky-patching-role",
				Labels: map[string]string{
					"app.kubernetes.io/name":       "redskyops",
					"app.kubernetes.io/version":    version.Version,
					"app.kubernetes.io/managed-by": "redskyctl",
				},
			},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"redskyops.dev/aggregate-to-patching": "true"}},
				},
			},
		},
		PatchingClusterRoleBinding: rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "redsky-patching-rolebinding",
				Labels: map[string]string{
					"app.kubernetes.io/name":       "redskyops",
					"app.kubernetes.io/version":    version.Version,
					"app.kubernetes.io/managed-by": "redskyctl",
				},
			},
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
				Name:     "redsky-patching-role",
			},
		},

		// The default patching cluster role determines which objects can be patched for a trial by default, customers may modify or even delete this role
		DefaultPatchingClusterRole: rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "redsky-aggregate-to-patching",
				Labels: map[string]string{
					// Do not give this an application name label to prevent modified instances from being pruned during init
					"redskyops.dev/aggregate-to-patching": "true",
				},
			},
			Rules: []rbacv1.PolicyRule{
				// "patch" is used to actually apply the patch
				// "get" is used to check for stability
				{
					Verbs:     []string{"patch", "get"},
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
				},
				{
					Verbs:     []string{"patch", "get"},
					APIGroups: []string{"apps", "extensions"},
					Resources: []string{"deployments", "statefulsets"},
				},
			},
		},
	}

	// Make sure the bootstrap cluster role has all the rules needed to create the default cluster role
	b.ClusterRole.Rules = append(b.ClusterRole.Rules, b.DefaultPatchingClusterRole.Rules...)

	// Create, but do not execute the job
	if o.Bootstrap && !o.DryRun {
		b.Job.Spec.Parallelism = new(int32)
	}

	// Request generation of manifests only
	if o.DryRun && !o.Bootstrap {
		b.Job.Spec.Template.Spec.Containers[0].Args = append(b.Job.Spec.Template.Spec.Containers[0].Args, "--dry-run")
	}

	if o.ClientSet != nil {
		applyClientSet(b, o)
	}
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
