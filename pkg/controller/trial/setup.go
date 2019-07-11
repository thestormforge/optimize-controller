package trial

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	redskyv1alpha1 "github.com/gramLabs/redsky/pkg/apis/redsky/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	// This is overwritten during builds to point to the actual image
	DefaultImage = "setuptools:latest"
)

const (
	setupFinalizer = "setupFinalizer.redsky.carbonrelay.com"

	// These are the arguments accepted by the setuptools container
	modeCreate = "create"
	modeDelete = "delete"

	// A job that cannot start within this timeout is considered failed
	// This is a workaround for not checking the pod status or events that may indicate why the job isn't started
	// One common reason for a job not starting is that the setup service account does not exist
	startTimeout = 2 * time.Minute
)

func manageSetup(c client.Client, s *runtime.Scheme, trial *redskyv1alpha1.Trial) (reconcile.Result, bool, error) {
	// Determine if there is anything to do
	probeTime := metav1.Now()
	if probeSetupTrialConditions(trial, &probeTime) {
		return reconcile.Result{}, false, nil
	}

	// Update the conditions based on existing jobs
	list := &batchv1.JobList{}
	setupJobLabels := map[string]string{"role": "trialSetup", "setupFor": trial.Name}
	if err := c.List(context.TODO(), list, client.MatchingLabels(setupJobLabels)); err != nil {
		return reconcile.Result{}, false, err
	}
	for i := range list.Items {
		job := &list.Items[i]
		conditionType, err := findSetupJobConditionType(job)
		if err != nil {
			return reconcile.Result{}, false, err
		}

		// If any setup job failed there is nothing more to do but mark the whole trial failed (if we haven't already)
		if failed, message := isSetupJobFailed(job); failed {
			if removeSetupFinalizer(trial) && !IsTrialFinished(trial) {
				trial.Status.Conditions = append(trial.Status.Conditions, redskyv1alpha1.TrialCondition{
					Type:               redskyv1alpha1.TrialFailed,
					Status:             corev1.ConditionTrue,
					LastProbeTime:      probeTime,
					LastTransitionTime: probeTime,
					Reason:             "SetupJobFailed",
					Message:            message,
				})
				break // Stop the loop, will require an update
			}
			return reconcile.Result{}, false, nil
		}

		// Update the condition associated with this job
		setSetupTrialCondition(trial, conditionType, isSetupJobComplete(job), "", "")
	}

	// Check to see if we need to update the trial to record a condition change
	if needsUpdate(trial, &probeTime) {
		err := c.Update(context.TODO(), trial)
		return reconcile.Result{}, true, err
	}

	// Figure out if we need to start a job
	mode := ""

	// If the created condition is unknown, we will need a create job
	if cc, ok := checkSetupTrialCondition(trial, redskyv1alpha1.TrialSetupCreated, corev1.ConditionUnknown); cc && ok {
		mode = modeCreate
	}

	// If the deleted condition is unknown, we may need a delete job
	if cc, ok := checkSetupTrialCondition(trial, redskyv1alpha1.TrialSetupDeleted, corev1.ConditionUnknown); cc && ok {
		if addSetupFinalizer(trial) {
			err := c.Update(context.TODO(), trial)
			return reconcile.Result{}, true, err
		}

		if IsTrialFinished(trial) || trial.DeletionTimestamp != nil {
			mode = modeDelete
		}
	}

	// Create a setup job if necessary
	if mode != "" {
		job, err := newSetupJob(trial, s, mode)
		if err != nil {
			return reconcile.Result{}, false, err
		}
		err = c.Create(context.TODO(), job)
		return reconcile.Result{}, true, err
	}

	// If the create job isn't finished, wait for it
	if cc, ok := checkSetupTrialCondition(trial, redskyv1alpha1.TrialSetupCreated, corev1.ConditionFalse); cc && ok {
		return reconcile.Result{Requeue: true, RequeueAfter: 1 * time.Second}, false, nil
	}

	// If the delete job is finished, remove our finalizer
	if cc, ok := checkSetupTrialCondition(trial, redskyv1alpha1.TrialSetupDeleted, corev1.ConditionTrue); cc && ok {
		if removeSetupFinalizer(trial) {
			err := c.Update(context.TODO(), trial)
			return reconcile.Result{}, true, err
		}
	}

	// There are no setup task actions to perform
	return reconcile.Result{}, false, nil
}

func addSetupFinalizer(trial *redskyv1alpha1.Trial) bool {
	for _, f := range trial.Finalizers {
		if f == setupFinalizer {
			return false
		}
	}
	trial.Finalizers = append(trial.Finalizers, setupFinalizer)
	return true
}

func removeSetupFinalizer(trial *redskyv1alpha1.Trial) bool {
	for i := range trial.Finalizers {
		if trial.Finalizers[i] == setupFinalizer {
			trial.Finalizers[i] = trial.Finalizers[len(trial.Finalizers)-1]
			trial.Finalizers = trial.Finalizers[:len(trial.Finalizers)-1]
			return true
		}
	}
	return false
}

func isSetupJobComplete(job *batchv1.Job) corev1.ConditionStatus {
	// We MUST return either True or False; Unknown has special meaning
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return corev1.ConditionTrue
		}
	}
	return corev1.ConditionFalse
}

func isSetupJobFailed(job *batchv1.Job) (bool, string) {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			m := c.Message
			if m == "" && c.Reason != "" {
				m = fmt.Sprintf("Setup job failed with reason '%s'", c.Reason)
			}
			if m == "" {
				m = "Setup job failed without reporting a reason"
			}
			return true, m
		}
	}

	// For versions of Kube that do not report failures as conditions, just look for failed pods
	if job.Status.Failed > 0 {
		return true, fmt.Sprintf("Setup job has %d failed pod(s)", job.Status.Failed)
	}

	// It's possible the job isn't being run. Pretend it finished if it hasn't started in time
	if job.Status.Succeeded == 0 && job.Status.Failed == 0 && job.Status.Active == 0 {
		if metav1.Now().Sub(job.CreationTimestamp.Time) > startTimeout {
			return true, "Setup job failed to start"
		}
	}

	// TODO We may need to check pod status if active > 0

	return false, ""
}

func findSetupJobConditionType(job *batchv1.Job) (redskyv1alpha1.TrialConditionType, error) {
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

func setSetupTrialCondition(trial *redskyv1alpha1.Trial, conditionType redskyv1alpha1.TrialConditionType, status corev1.ConditionStatus, reason, message string) {
	for i := range trial.Status.Conditions {
		if trial.Status.Conditions[i].Type == conditionType {
			if trial.Status.Conditions[i].Status != status {
				trial.Status.Conditions[i].Status = status
				trial.Status.Conditions[i].Reason = reason
				trial.Status.Conditions[i].Message = message
				trial.Status.Conditions[i].LastTransitionTime = trial.Status.Conditions[i].LastProbeTime
			}
			break
		}
	}
}

func checkSetupTrialCondition(trial *redskyv1alpha1.Trial, conditionType redskyv1alpha1.TrialConditionType, status corev1.ConditionStatus) (bool, bool) {
	for i := range trial.Status.Conditions {
		if trial.Status.Conditions[i].Type == conditionType {
			return trial.Status.Conditions[i].Status == status, true
		}
	}
	return false, false
}

func needsUpdate(trial *redskyv1alpha1.Trial, probeTime *metav1.Time) bool {
	for i := range trial.Status.Conditions {
		// TODO Can we use pointer equivalence here? Might be a more accurate reflection of what we are trying to do
		if trial.Status.Conditions[i].LastTransitionTime.Equal(probeTime) {
			return true
		}
	}
	return false
}

func newSetupJob(trial *redskyv1alpha1.Trial, scheme *runtime.Scheme, mode string) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	job.Namespace = trial.Namespace
	job.Name = fmt.Sprintf("%s-%s", trial.Name, mode)
	job.Labels = map[string]string{"role": "trialSetup", "setupFor": trial.Name}
	job.Spec.BackoffLimit = new(int32)
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
			},
		}

		// Make sure we have an image
		if c.Image == "" {
			c.Image = DefaultImage
		}

		// If this appears to be a development image, change the image pull policy
		if !strings.Contains(c.Image, "/") {
			c.ImagePullPolicy = corev1.PullIfNotPresent
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

		// For Helm installs, include the chart name and options for setting values
		if task.HelmChart != "" {
			helmOpts, err := generateHelmOptions(trial, &task)
			if err != nil {
				return nil, err
			}
			helmOpts = append(helmOpts, "--namespace", namespace)

			c.Env = append(c.Env, corev1.EnvVar{Name: "CHART", Value: task.HelmChart})
			c.Env = append(c.Env, corev1.EnvVar{Name: "HELM_OPTS", Value: strings.Join(helmOpts, " ")})

			for _, hvf := range task.HelmValuesFrom {
				// TODO Since this is "HelmValuesFrom", do we need to somehow limit keys to "*values.yaml"?
				if hvf.ConfigMap != nil {
					c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
						Name:      hvf.ConfigMap.Name,
						MountPath: path.Join("/workspace", "helm", hvf.ConfigMap.Name),
						ReadOnly:  true,
					})
					if v, ok := volumes[hvf.ConfigMap.Name]; !ok {
						volumes[hvf.ConfigMap.Name] = &corev1.Volume{
							Name: hvf.ConfigMap.Name,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: hvf.ConfigMap.Name,
									},
								},
							},
						}
					} else if v.ConfigMap == nil {
						return nil, fmt.Errorf("expected configMap volume for %s", v.Name)
					}
				}
			}
		}

		job.Spec.Template.Spec.Containers = append(job.Spec.Template.Spec.Containers, c)
	}

	// Add all of the volumes we collected to the pod
	for _, v := range volumes {
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, *v)
	}

	// Set the owner of the job to the trial
	if err := controllerutil.SetControllerReference(trial, job, scheme); err != nil {
		return nil, err
	}

	return job, nil
}

func generateHelmOptions(trial *redskyv1alpha1.Trial, task *redskyv1alpha1.SetupTask) ([]string, error) {
	var opts []string

	// NOTE: Since the content of the ConfigMaps is dynamic, we only look for --values files from the running container

	// Add individual --set options
	for _, hv := range task.HelmValues {
		if hv.ForceString {
			opts = append(opts, "--set-string")
		} else {
			opts = append(opts, "--set")
		}

		if hv.ValueFrom != nil {
			// Evaluate the external value source
			switch {
			case hv.ValueFrom.ParameterRef != nil:
				if v, ok := trial.GetAssignment(hv.ValueFrom.ParameterRef.Name); ok {
					opts = append(opts, fmt.Sprintf(`"%s=%d"`, hv.Name, v))
				} else {
					return nil, fmt.Errorf("invalid parameter reference '%s' for Helm value '%s'", hv.ValueFrom.ParameterRef.Name, hv.Name)
				}
			default:
				return nil, fmt.Errorf("unknown source for Helm value '%s'", hv.Name)
			}
		} else {
			// If there is no external source, evaluate the value field as a template
			if v, err := executeAssignmentTemplate(hv.Value.String(), trial); err == nil {
				opts = append(opts, fmt.Sprintf(`"%s=%s"`, hv.Name, v))
			} else {
				return nil, err
			}
		}
	}

	// Use the task name as the Helm release name
	opts = append(opts, "--name", task.Name)

	return opts, nil
}
