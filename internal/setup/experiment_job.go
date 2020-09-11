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

package setup

import (
	"fmt"
	"os"

	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

// NOTE: The default image names use a ":latest" tag which causes the default pull policy to switch
// from "IfNotPresent" to "Always". However, the default image names are not associated with a public
// repository and cannot actually be pulled (they only work if they are present). The exact opposite
// problem occurs with the production image names: we want those to have a policy of "Always" to address
// the potential of a floating tag but they will default to "IfNotPresent" because they do not use
// ":latest". To address this we always explicitly specify the pull policy corresponding to the image.
// Finally, when using digests, the default of "IfNotPresent" is acceptable as it is unambiguous.

// NewTrialJob returns a new setup job for either create or delete
func NewExperimentJob(exp *redskyv1beta1.Experiment, mode string) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	job.Namespace = exp.Namespace
	job.Name = fmt.Sprintf("%s-%s", exp.Name, mode)

	job.Labels = map[string]string{
		redskyv1beta1.LabelExperiment: exp.Name,
	}

	job.Spec.BackoffLimit = new(int32)
	job.Spec.Template.Labels = map[string]string{
		redskyv1beta1.LabelExperiment:     exp.Name,
		redskyv1beta1.LabelExperimentRole: "experimentSetup",
	}
	job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

	// TODO: not sure which service account we want to use with this,
	// maybe piggyback off trial spec service account?
	if exp.Spec.TrialTemplate.Spec.SetupServiceAccountName != "" {
		job.Spec.Template.Spec.ServiceAccountName = exp.Spec.TrialTemplate.Spec.SetupServiceAccountName
	}

	runAsNonRoot := true
	job.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
	}

	// We need to run as a non-root user that has the same UID and GID
	id := int64(1000)
	allowPrivilegeEscalation := false

	c := corev1.Container{
		Name: fmt.Sprintf("%s-%s", job.Name, "prometheus-setup"),
		Args: []string{"prometheus", mode},
		Env: []corev1.EnvVar{
			{Name: "NAMESPACE", Value: exp.Namespace},
			{Name: "NAME", Value: fmt.Sprintf("%s-%s", job.Name, "prometheus-setup")},
			{Name: "EXPERIMENT", Value: exp.Name},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:                &id,
			RunAsGroup:               &id,
			AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		},
	}

	// Check the environment for a default setup tools image name
	if c.Image == "" {
		c.Image = os.Getenv("DEFAULT_SETUP_IMAGE")
		c.ImagePullPolicy = corev1.PullPolicy(os.Getenv("DEFAULT_SETUP_IMAGE_PULL_POLICY"))
	}

	// Make sure we have an image
	if c.Image == "" {
		c.Image = Image
		c.ImagePullPolicy = corev1.PullPolicy(ImagePullPolicy)
	}

	job.Spec.Template.Spec.Containers = append(job.Spec.Template.Spec.Containers, c)

	return job, nil
}
