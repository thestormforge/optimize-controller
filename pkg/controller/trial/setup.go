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

package trial

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/redskyops/k8s-experiment/internal/trial"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	"github.com/redskyops/k8s-experiment/pkg/controller/template"
	"github.com/redskyops/k8s-experiment/pkg/util"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

var (
	// This is overwritten during builds to point to the actual image
	Image                  = "setuptools:latest"
	ImagePullPolicy string = string(corev1.PullIfNotPresent)
)

// NOTE: The default image names use a ":latest" tag which causes the default pull policy to switch
// from "IfNotPresent" to "Always". However, the default image names are not associated with a public
// repository and cannot actually be pulled (they only work if they are present). The exact opposite
// problem occurs with the production image names: we want those to have a policy of "Always" to address
// the potential of a floating tag but they will default to "IfNotPresent" because they do not use
// ":latest". To address this we always explicitly specify the pull policy corresponding to the image.
// Finally, when using digests, the default of "IfNotPresent" is acceptable as it is unambiguous.

const (
	setupFinalizer = "setupFinalizer.redskyops.dev"

	// These are the arguments accepted by the setuptools container
	modeCreate = "create"
	modeDelete = "delete"
)

func ManageSetup(c client.Client, s *runtime.Scheme, ctx context.Context, probeTime *metav1.Time, t *redskyv1alpha1.Trial) (*ctrl.Result, error) {
	// Determine if there is anything to do
	if probeSetupTrialConditions(t, probeTime) {
		return nil, nil
	}

	// TODO Have a setting to force this to return here so setup tasks aren't actually evaluated (need to update conditions with "disabled")

	// Find the setup jobs for this trial
	list := &batchv1.JobList{}
	setupJobLabels := map[string]string{redskyv1alpha1.LabelTrial: t.Name, redskyv1alpha1.LabelTrialRole: "trialSetup"}
	if err := c.List(ctx, list, client.InNamespace(t.Namespace), client.MatchingLabels(setupJobLabels)); err != nil {
		return &ctrl.Result{}, err
	}

	// This is purely for recovery, normally if the list size is zero the condition status will already be "unknown"
	if len(list.Items) == 0 {
		ApplyCondition(&t.Status, redskyv1alpha1.TrialSetupCreated, corev1.ConditionUnknown, "", "", probeTime)
		ApplyCondition(&t.Status, redskyv1alpha1.TrialSetupDeleted, corev1.ConditionUnknown, "", "", probeTime)
	}

	// Update the conditions based on existing jobs
	for i := range list.Items {
		job := &list.Items[i]

		// Inspect the job to determine which condition to update
		conditionType, err := getSetupJobType(job)
		if err != nil {
			return &ctrl.Result{}, err
		}

		// Determine if the job is finished (i.e. completed or failed)
		conditionStatus, failureMessage := getSetupJobStatus(c, ctx, job)
		ApplyCondition(&t.Status, conditionType, conditionStatus, "", "", probeTime)

		// Only fail the trial itself if it isn't already finished; both to prevent overwriting an existing success
		// or failure status and to avoid updating the probe time (which would get us stuck in a busy loop)
		if failureMessage != "" && !trial.IsFinished(t) {
			ApplyCondition(&t.Status, redskyv1alpha1.TrialFailed, corev1.ConditionTrue, "SetupJobFailed", failureMessage, probeTime)
		}
	}

	// Check to see if we need to update the trial to record a condition change
	// TODO This check just looks for the probeTime in "last transition" times, is this causing unnecessary updates?
	// TODO Can we use pointer equivalence on probeTime to help mitigate that problem?
	if needsUpdate(t, probeTime) {
		err := c.Update(ctx, t)
		rr, re := util.RequeueConflict(ctrl.Result{}, err)
		return &rr, re
	}

	// Figure out if we need to start a job
	mode := ""

	// If the created condition is unknown, we will need a create job
	if cc, ok := CheckCondition(&t.Status, redskyv1alpha1.TrialSetupCreated, corev1.ConditionUnknown); cc && ok {
		// Before we can have a create job, we need a finalizer so we get a chance to run the delete job
		if addSetupFinalizer(t) {
			err := c.Update(ctx, t)
			return &ctrl.Result{}, err
		}

		mode = modeCreate
	}

	// If the deleted condition is unknown, we may need a delete job
	if cc, ok := CheckCondition(&t.Status, redskyv1alpha1.TrialSetupDeleted, corev1.ConditionUnknown); cc && ok {
		// We do not need the deleted job until the trial is finished or it gets deleted
		if trial.IsFinished(t) || !t.DeletionTimestamp.IsZero() {
			mode = modeDelete
		}
	}

	// Create a setup job if necessary
	if mode != "" {
		job, err := newSetupJob(t, mode)
		if err != nil {
			return &ctrl.Result{}, err
		}
		if err := controllerutil.SetControllerReference(t, job, s); err != nil {
			return nil, err
		}
		err = c.Create(ctx, job)
		return &ctrl.Result{}, err
	}

	// If the create job isn't finished, wait for it (unless the trial is already finished, i.e. failed)
	if cc, ok := CheckCondition(&t.Status, redskyv1alpha1.TrialSetupCreated, corev1.ConditionFalse); ok && cc {
		if !trial.IsFinished(t) && t.DeletionTimestamp.IsZero() {
			return &ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}
	}

	// Do not remove the finalizer until the delete job is finished
	if cc, ok := CheckCondition(&t.Status, redskyv1alpha1.TrialSetupDeleted, corev1.ConditionTrue); ok && cc {
		if util.RemoveFinalizer(t, setupFinalizer) {
			err := c.Update(ctx, t)
			return &ctrl.Result{}, err
		}
	}

	// There are no setup task actions to perform
	return nil, nil
}

func addSetupFinalizer(trial *redskyv1alpha1.Trial) bool {
	// TODO Will this add a finalizer to a deleted trial? If not, use the util.AddFinalizer instead
	for _, f := range trial.Finalizers {
		if f == setupFinalizer {
			return false
		}
	}
	trial.Finalizers = append(trial.Finalizers, setupFinalizer)
	return true
}

func getSetupJobType(job *batchv1.Job) (redskyv1alpha1.TrialConditionType, error) {
	// TODO This should just be a label or annotation on the job
	for _, c := range job.Spec.Template.Spec.Containers {
		if len(c.Args) > 0 {
			switch c.Args[0] {
			case modeCreate:
				return redskyv1alpha1.TrialSetupCreated, nil
			case modeDelete:
				return redskyv1alpha1.TrialSetupDeleted, nil
			default:
				return "", fmt.Errorf("unknown setup job container argument: %s", c.Args[0])
			}
		}
	}
	return "", fmt.Errorf("unable to determine setup job type")
}

func getSetupJobStatus(c client.Client, ctx context.Context, job *batchv1.Job) (corev1.ConditionStatus, string) {
	// Check the job conditions first
	for _, c := range job.Status.Conditions {
		if c.Status == corev1.ConditionTrue {
			switch c.Type {
			case batchv1.JobComplete:
				return corev1.ConditionTrue, ""
			case batchv1.JobFailed:
				switch c.Reason {
				case "BackoffLimitExceeded":
					// If we hit the backoff limit it means that at least one container is exiting with 1
					return corev1.ConditionTrue, "Setup job did not complete successfully"
				default:
					// Use the condition to construct a message
					m := c.Message
					if m == "" && c.Reason != "" {
						m = fmt.Sprintf("Setup job failed with reason '%s'", c.Reason)
					}
					if m == "" {
						m = "Setup job failed without reporting a reason"
					}
					return corev1.ConditionTrue, m
				}
			}
		}
	}

	// For versions of Kube that do not report failures as conditions, just look for failed pods
	if job.Status.Failed > 0 {
		return corev1.ConditionTrue, fmt.Sprintf("Setup job has %d failed pod(s)", job.Status.Failed)
	}

	// As a last resort try checking the job pods
	list := &corev1.PodList{}
	if matchingSelector, err := util.MatchingSelector(job.Spec.Selector); err == nil {
		_ = c.List(ctx, list, client.InNamespace(job.Namespace), matchingSelector)
	}
	for i := range list.Items {
		for _, cs := range list.Items[i].Status.ContainerStatuses {
			if !cs.Ready && cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
				return corev1.ConditionTrue, fmt.Sprintf("Setup job has a failed container")
			}
		}
	}

	return corev1.ConditionFalse, ""
}

// Returns true if the setup tasks are done
func probeSetupTrialConditions(trial *redskyv1alpha1.Trial, probeTime *metav1.Time) bool {
	var needsCreate, needsDelete bool
	for _, t := range trial.Spec.SetupTasks {
		needsCreate = needsCreate || !t.SkipCreate
		needsDelete = needsDelete || !t.SkipDelete
	}

	// Short circuit, there are no setup tasks
	if !needsCreate && !needsDelete {
		return true
	}

	// TODO Can we return true from this loop as an optimization if the status is True?
	for i := range trial.Status.Conditions {
		switch trial.Status.Conditions[i].Type {
		case redskyv1alpha1.TrialSetupCreated:
			trial.Status.Conditions[i].LastProbeTime = *probeTime
			needsCreate = false
		case redskyv1alpha1.TrialSetupDeleted:
			trial.Status.Conditions[i].LastProbeTime = *probeTime
			needsDelete = false
		}
	}

	if needsCreate {
		trial.Status.Conditions = append(trial.Status.Conditions, redskyv1alpha1.TrialCondition{
			Type:               redskyv1alpha1.TrialSetupCreated,
			Status:             corev1.ConditionUnknown,
			LastProbeTime:      *probeTime,
			LastTransitionTime: *probeTime,
		})
	}

	if needsDelete {
		trial.Status.Conditions = append(trial.Status.Conditions, redskyv1alpha1.TrialCondition{
			Type:               redskyv1alpha1.TrialSetupDeleted,
			Status:             corev1.ConditionUnknown,
			LastProbeTime:      *probeTime,
			LastTransitionTime: *probeTime,
		})
	}

	// There is at least one setup task
	return false
}

func needsUpdate(trial *redskyv1alpha1.Trial, probeTime *metav1.Time) bool {
	for i := range trial.Status.Conditions {
		if trial.Status.Conditions[i].LastTransitionTime.Equal(probeTime) {
			return true
		}
	}
	return false
}

func newSetupJob(trial *redskyv1alpha1.Trial, mode string) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	job.Namespace = trial.Namespace
	job.Name = fmt.Sprintf("%s-%s", trial.Name, mode)
	job.Labels = map[string]string{
		redskyv1alpha1.LabelExperiment: trial.ExperimentNamespacedName().Name,
		redskyv1alpha1.LabelTrial:      trial.Name,
		redskyv1alpha1.LabelTrialRole:  "trialSetup",
	}
	job.Spec.BackoffLimit = new(int32)
	job.Spec.Template.Labels = map[string]string{
		redskyv1alpha1.LabelExperiment: trial.ExperimentNamespacedName().Name,
		redskyv1alpha1.LabelTrial:      trial.Name,
		redskyv1alpha1.LabelTrialRole:  "trialSetup",
	}
	job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	job.Spec.Template.Spec.ServiceAccountName = trial.Spec.SetupServiceAccountName

	// Collect the volumes we need for the pod
	var volumes = make(map[string]*corev1.Volume)
	for _, v := range trial.Spec.SetupVolumes {
		volumes[v.Name] = &v
	}

	// Determine the namespace to operate in
	namespace := trial.Spec.TargetNamespace
	if namespace == "" {
		namespace = trial.Namespace
	}

	// We need to run as a non-root user that has the same UID and GID
	id := int64(1000)
	allowPrivilegeEscalation := false
	runAsNonRoot := true
	job.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
	}

	// Create containers for each of the setup tasks
	for _, task := range trial.Spec.SetupTasks {
		if (mode == modeCreate && task.SkipCreate) || (mode == modeDelete && task.SkipDelete) {
			continue
		}
		c := corev1.Container{
			Name:  fmt.Sprintf("%s-%s", job.Name, task.Name),
			Image: task.Image,
			Args:  []string{mode},
			Env: []corev1.EnvVar{
				{Name: "NAMESPACE", Value: namespace},
				{Name: "NAME", Value: task.Name},
				{Name: "TRIAL", Value: trial.Name},
			},
			SecurityContext: &corev1.SecurityContext{
				RunAsUser:                &id,
				RunAsGroup:               &id,
				AllowPrivilegeEscalation: &allowPrivilegeEscalation,
			},
		}

		// Make sure we have an image
		if c.Image == "" {
			c.Image = Image
			c.ImagePullPolicy = corev1.PullPolicy(ImagePullPolicy)
		}

		// Add the trial assignments to the environment
		for _, a := range trial.Spec.Assignments {
			name := strings.ReplaceAll(strings.ToUpper(a.Name), ".", "_")
			c.Env = append(c.Env, corev1.EnvVar{Name: name, Value: fmt.Sprintf("%d", a.Value)})
		}

		// Add the configured volume mounts
		for _, vm := range task.VolumeMounts {
			c.VolumeMounts = append(c.VolumeMounts, vm)
		}

		// For Helm installs, serialize a Konjure configuration
		helmConfig := NewHelmGeneratorConfig(&task)
		if helmConfig != nil {
			te := template.NewTemplateEngine()

			// Helm Values
			for _, hv := range task.HelmValues {
				hgv := HelmGeneratorValue{
					Name:        hv.Name,
					ForceString: hv.ForceString,
				}

				if hv.ValueFrom != nil {
					// Evaluate the external value source
					switch {
					case hv.ValueFrom.ParameterRef != nil:
						v, ok := trial.GetAssignment(hv.ValueFrom.ParameterRef.Name)
						if !ok {
							return nil, fmt.Errorf("invalid parameter reference '%s' for Helm value '%s'", hv.ValueFrom.ParameterRef.Name, hv.Name)
						}
						hgv.Value = v

					default:
						return nil, fmt.Errorf("unknown source for Helm value '%s'", hv.Name)
					}
				} else {
					// If there is no external source, evaluate the value field as a template
					v, err := te.RenderHelmValue(&hv, trial)
					if err != nil {
						return nil, err
					}
					hgv.Value = v
				}

				helmConfig.Values = append(helmConfig.Values, hgv)
			}

			// Helm Values From
			for _, hvf := range task.HelmValuesFrom {
				if hvf.ConfigMap != nil {
					hgv := HelmGeneratorValue{
						File: path.Join("/workspace", "helm-values", hvf.ConfigMap.Name, "*values.yaml"),
					}
					vm := corev1.VolumeMount{
						Name:      hvf.ConfigMap.Name,
						MountPath: path.Dir(hgv.File),
						ReadOnly:  true,
					}

					if _, ok := volumes[vm.Name]; !ok {
						vs := corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: hvf.ConfigMap.Name},
							},
						}
						volumes[vm.Name] = &corev1.Volume{Name: vm.Name, VolumeSource: vs}
					}
					c.VolumeMounts = append(c.VolumeMounts, vm)
					helmConfig.Values = append(helmConfig.Values, hgv)
				}
			}

			// Record the base64 encoded YAML representation in the environment
			b, err := yaml.Marshal(helmConfig)
			if err != nil {
				return nil, err
			}
			c.Env = append(c.Env, corev1.EnvVar{Name: "HELM_CONFIG", Value: base64.StdEncoding.EncodeToString(b)})
		}

		job.Spec.Template.Spec.Containers = append(job.Spec.Template.Spec.Containers, c)
	}

	// Add all of the volumes we collected to the pod
	for _, v := range volumes {
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, *v)
	}

	return job, nil
}

type HelmGeneratorValue struct {
	File        string      `json:"file,omitempty"`
	Name        string      `json:"name,omitempty"`
	Value       interface{} `json:"value,omitempty"`
	ForceString bool        `json:"forceString,omitempty"`
}

type HelmGeneratorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	ReleaseName       string               `json:"releaseName"`
	Chart             string               `json:"chart"`
	Version           string               `json:"version"`
	Values            []HelmGeneratorValue `json:"values"`
}

func NewHelmGeneratorConfig(task *redskyv1alpha1.SetupTask) *HelmGeneratorConfig {
	if task.HelmChart == "" {
		return nil
	}

	cfg := &HelmGeneratorConfig{
		ReleaseName: task.Name,
		Chart:       task.HelmChart,
		Version:     task.HelmChartVersion,
	}

	cfg.APIVersion = "konjure.carbonrelay.com/v1beta1"
	cfg.Kind = "HelmGenerator"
	cfg.Name = task.Name

	return cfg
}
