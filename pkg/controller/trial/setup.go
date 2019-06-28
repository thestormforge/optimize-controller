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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	DefaultImage = "setuptools:latest"
)

const (
	setupFinalizer = "setupFinalizer.redsky.carbonrelay.com"
	create         = "create"
	delete         = "delete"
)

func manageSetup(c client.Client, s *runtime.Scheme, trial *redskyv1alpha1.Trial) (reconcile.Result, bool, error) {
	// Determine which jobs are initially required
	var needsCreate, needsDelete, finishedCreate, finishedDelete bool
	for _, t := range trial.Spec.SetupTasks {
		needsCreate = needsCreate || !t.SkipCreate
		needsDelete = needsDelete || !t.SkipDelete
	}
	if !needsCreate && !needsDelete {
		return reconcile.Result{}, false, nil
	}

	// We do not need a delete job if the trial is still in progress and has not been deleted
	if !IsTrialFinished(trial) && trial.DeletionTimestamp == nil {
		if needsDelete && addSetupFinalizer(trial) {
			err := c.Update(context.TODO(), trial)
			return reconcile.Result{}, true, err
		}
		needsDelete = false
	}

	// Update which jobs are required based on the existing jobs
	list := &batchv1.JobList{}
	setupJobLabels := map[string]string{"role": "trialSetup", "setupFor": trial.Name}
	if err := c.List(context.TODO(), list, client.MatchingLabels(setupJobLabels)); err != nil {
		return reconcile.Result{}, false, err
	}
	for _, j := range list.Items {
		for _, c := range j.Spec.Template.Spec.Containers {
			if len(c.Args) > 0 {
				switch c.Args[0] {
				case create:
					needsCreate = false
					finishedCreate = isJobFinished(&j)
				case delete:
					needsDelete = false
					finishedDelete = isJobFinished(&j)
				}
				break
			}
		}
	}

	// Create any jobs that are required
	if needsCreate || needsDelete {
		mode := delete
		if needsCreate {
			mode = create
		}

		job, err := newSetupJob(trial, s, mode)
		if err != nil {
			return reconcile.Result{}, false, err
		}
		err = c.Create(context.TODO(), job)
		return reconcile.Result{}, true, err
	}

	// If the create job isn't finished, wait for it
	if !finishedCreate {
		return reconcile.Result{Requeue: true, RequeueAfter: 1 * time.Second}, false, nil
	}

	// If the delete job is finished remove our finalizer
	if finishedDelete {
		for i := range trial.Finalizers {
			if trial.Finalizers[i] == setupFinalizer {
				trial.Finalizers[i] = trial.Finalizers[len(trial.Finalizers)-1]
				trial.Finalizers = trial.Finalizers[:len(trial.Finalizers)-1]
				err := c.Update(context.TODO(), trial)
				return reconcile.Result{}, true, err
			}
		}
	}

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

func isJobFinished(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	if job.Status.Failed > 0 {
		return true
	}
	return false
}

func newSetupJob(trial *redskyv1alpha1.Trial, scheme *runtime.Scheme, mode string) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	job.Namespace = trial.Namespace
	job.Name = fmt.Sprintf("%s-%s", trial.Name, mode)
	job.Labels = map[string]string{"role": "trialSetup", "setupFor": trial.Name}
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
		if (mode == create && task.SkipCreate) || (mode == delete && task.SkipDelete) {
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

	// Add individual --set* options
	// TODO Do we need to support --set-string?
	for _, hv := range task.HelmValues {
		v := hv.Value.String()
		if v != "" {
			v, err := executeAssignmentTemplate(v, trial)
			if err != nil {
				return nil, err
			}
			opts = append(opts, "--set", fmt.Sprintf("%s=%s", hv.Name, v))
		} else if hv.ValueFrom != nil {
			switch {
			// TODO We should support the other environment variable sources for this as well
			case hv.ValueFrom.ParameterRef != nil:
				for _, a := range trial.Spec.Assignments {
					if a.Name == hv.ValueFrom.ParameterRef.Name {
						opts = append(opts, "--set", fmt.Sprintf("%s=%d", hv.Name, a.Value))
						break
					}
				}
			}
		}
	}

	// Use the task name as the Helm release name
	opts = append(opts, "--name", task.Name)

	return opts, nil
}
