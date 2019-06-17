package trial

import (
	"context"
	"fmt"
	"strings"
	"time"

	cordeliav1alpha1 "github.com/gramLabs/cordelia/pkg/apis/cordelia/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	defaultImage   string = "setuptools:latest"
	setupFinalizer        = "setupFinalizer.cordelia.carbonrelay.com"
	create                = "create"
	delete                = "delete"
)

func manageSetup(c client.Client, s *runtime.Scheme, trial *cordeliav1alpha1.Trial) (reconcile.Result, bool, error) {
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
			for _, e := range c.Env {
				if e.Name == "MODE" {
					switch e.Value {
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
			break // All containers have the same environment
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

func addSetupFinalizer(trial *cordeliav1alpha1.Trial) bool {
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

func newSetupJob(trial *cordeliav1alpha1.Trial, scheme *runtime.Scheme, mode string) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	job.Namespace = trial.Namespace
	job.Name = fmt.Sprintf("%s-%s", trial.Name, mode)
	job.Labels = map[string]string{"role": "trialSetup", "setupFor": trial.Name}

	job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	job.Spec.Template.Spec.Volumes = trial.Spec.SetupVolumes
	job.Spec.Template.Spec.ServiceAccountName = trial.Spec.SetupServiceAccountName

	for _, task := range trial.Spec.SetupTasks {
		if (mode == create && task.SkipCreate) || (mode == delete && task.SkipDelete) {
			continue
		}

		// Determine the namespace to operate in
		namespace := trial.Spec.TargetNamespace
		if namespace == "" {
			namespace = trial.Namespace
		}

		// Create a container with an environment that can be used to create or delete the required software
		c := corev1.Container{
			Name:  fmt.Sprintf("%s-%s", job.Name, task.Name),
			Image: task.Image,
			Env: []corev1.EnvVar{
				{Name: "MODE", Value: mode},
				{Name: "NAMESPACE", Value: namespace},
				{Name: "NAME", Value: task.Name},
			},
			VolumeMounts: task.VolumeMounts,
		}

		// Make sure we have an image
		if c.Image == "" {
			c.Image = defaultImage
			c.ImagePullPolicy = corev1.PullIfNotPresent // TODO Is this just for dev?
		}

		// Add the trial assignments to the environment
		for _, a := range trial.Spec.Assignments {
			name := strings.ReplaceAll(strings.ToUpper(a.Name), ".", "_")
			c.Env = append(c.Env, corev1.EnvVar{Name: name, Value: fmt.Sprintf("%d", a.Value)})
		}

		// For Helm installs, include the chart name
		if task.Chart != "" {
			c.Env = append(c.Env, corev1.EnvVar{Name: "CHART", Value: task.Chart})
		}

		job.Spec.Template.Spec.Containers = append(job.Spec.Template.Spec.Containers, c)
	}

	return job, controllerutil.SetControllerReference(trial, job, scheme)
}
