package cordeliactl

import (
	"io"
	"strings"

	"github.com/gramLabs/cordelia/pkg/api"
	"github.com/gramLabs/cordelia/pkg/controller/trial"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// The bootstrap configuration
type bootstrapConfig struct {
	Namespace          corev1.Namespace
	ClusterRole        rbacv1.ClusterRole
	ClusterRoleBinding rbacv1.ClusterRoleBinding
	Role               rbacv1.Role
	RoleBinding        rbacv1.RoleBinding
	Secret             corev1.Secret
	Job                batchv1.Job
}

func (b *bootstrapConfig) Marshal(w io.Writer) error {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

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
		if i > 0 {
			if _, err := w.Write([]byte("---\n")); err != nil {
				return err
			}
		}

		u := &unstructured.Unstructured{}
		err := scheme.Convert(objs[i], u, runtime.InternalGroupVersioner)
		if err != nil {
			return err
		}

		if b, err := yaml.Marshal(u); err != nil {
			return err
		} else if _, err := w.Write(b); err != nil {
			return err
		}
	}

	return nil
}

func newBootstrapConfig(namespace, name string, cfg *api.Config) (*bootstrapConfig, error) {
	clientConfig, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	// Note that we cannot scope "create" to a particular resource name

	b := &bootstrapConfig{
		// This is the namespace ultimately used by the product
		Namespace: corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
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
			Data:       map[string][]byte{"client.yaml": clientConfig},
		},

		// The job does a `kubectl apply` to the manifests of the product
		Job: batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{"app": "cordelia", "role": "install"},
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
										MountPath: "/cordelia/client/client.yaml",
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

	return b, nil
}
