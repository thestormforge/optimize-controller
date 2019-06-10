package trial

import (
	"context"
	"fmt"
	"time"

	okeanosv1alpha1 "github.com/gramLabs/okeanos/pkg/apis/okeanos/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	defaultImage string = "setuptools:latest"
	create              = "create"
	delete              = "delete"
)

func manageSetup(c client.Client, s *runtime.Scheme, trial *okeanosv1alpha1.Trial) (reconcile.Result, bool, error) {
	// Determine which jobs are initially required
	var needsCreate, needsDelete, finishedCreate bool
	for _, t := range trial.Spec.SetupTasks {
		needsCreate = needsCreate || !t.SkipCreate
		needsDelete = needsDelete || !t.SkipDelete
	}
	if !needsCreate && !needsDelete {
		return reconcile.Result{}, false, nil
	}

	// Update which jobs are required based on the existing jobs
	list := &batchv1.JobList{}
	setupJobLabels := map[string]string{"role": "trialSetup", "trial": trial.Name}
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
					}
					break
				}
			}
		}
	}

	// Create any jobs that are required
	if needsCreate || (needsDelete && IsTrialFinished(trial)) { // TODO Check trial deletion timestamp
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

	return reconcile.Result{}, false, nil
}

func isJobFinished(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func newSetupJob(trial *okeanosv1alpha1.Trial, scheme *runtime.Scheme, mode string) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	job.Namespace = trial.Namespace
	job.Name = fmt.Sprintf("%s-%s", trial.Name, mode)
	job.Labels = map[string]string{"role": "trialSetup", "trial": trial.Name}

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
			namespace = "default" // TODO Or should this be trial.Namespace? Needs to match what happens in the controller
		}

		// Determine a name that is going to be unique
		name := fmt.Sprintf("%s-%s", trial.Name, task.Name)

		// Create a container with an environment that can be used to create or delete the required software
		c := corev1.Container{
			Name:  name,
			Image: task.Image,
			Env: []corev1.EnvVar{
				{Name: "MODE", Value: mode},
				{Name: "NAMESPACE", Value: namespace},
				{Name: "NAME", Value: name},
			},
			VolumeMounts: task.VolumeMounts,
		}

		// Make sure we have an image
		if c.Image == "" {
			c.Image = defaultImage
			c.ImagePullPolicy = corev1.PullIfNotPresent // TODO Is this just for dev?
		}

		// Add the trial assignments to the environment
		for k, v := range trial.Spec.Assignments {
			// TODO Prefix assignment names? Adjust them to be upper-underscore?
			c.Env = append(c.Env, corev1.EnvVar{Name: k, Value: v})
		}

		// For Helm installs, include the chart name
		if task.Chart != "" {
			c.Env = append(c.Env, corev1.EnvVar{Name: "CHART", Value: task.Chart})
		}

		job.Spec.Template.Spec.Containers = append(job.Spec.Template.Spec.Containers, c)
	}

	return job, controllerutil.SetControllerReference(trial, job, scheme)
}
