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

package k8s

import (
	"fmt"
	"os"
	"strings"

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/kustomize/api/resmap"
)

// FindOrAddExperiment returns the experiment from the supplied list, creating it if it does not exist.
func FindOrAddExperiment(list *corev1.List) *redskyv1beta1.Experiment {
	var exp *redskyv1beta1.Experiment
	for i := range list.Items {
		if p, ok := list.Items[i].Object.(*redskyv1beta1.Experiment); ok {
			exp = p
			break
		}
	}

	if exp == nil {
		exp = &redskyv1beta1.Experiment{}
		list.Items = append(list.Items, runtime.RawExtension{Object: exp})
	}

	return exp
}

// EnsureSetupServiceAccount ensures that we are using an explicit service account for setup tasks.
func EnsureSetupServiceAccount(list *corev1.List) {
	// Return if we see an explicit service account name
	exp := FindOrAddExperiment(list)
	saName := &exp.Spec.TrialTemplate.Spec.SetupServiceAccountName
	if *saName != "" {
		return
	}
	*saName = "redsky-setup"

	// Add the actual service account to the list
	list.Items = append(list.Items,
		runtime.RawExtension{Object: &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: *saName,
			},
		}},
	)
}

// ScanForIngress returns the URL to use for ingress to the application
func ScanForIngress(rm resmap.ResMap, ingress *redskyappsv1alpha1.Ingress, external bool) (string, error) {
	// This needs to find the ingress.
	// I think there are 3 options:
	// 1. Poke around in the resource map
	// 2. Let the job query the Kube API at runtime
	// 3. Let the controller query the Kube API at runtime

	// For 1, we are unlikely to find much: we might be find some objects that have names that are later bound
	// into to something we can hit from outside the cluster (e.g. a host name rule on an ingress object)
	// but most of those will be ambiguous (or provider specific).

	// For 2, there is an RBAC problem: the job will need additional runtime permissions via it's service account
	// to query the Kube API.

	// For 3, the controller may have similar RBAC issues and is further limited by the fact that it is looking at
	// a generic job/trial template for clues about how to do the ingress lookup (at which time it would modify
	// the container definition for the job's pod, prior to creating the job). We might need some kind of Go template
	// in the environment variable values that the controller would evaluate prior to runtime (come to think of it,
	// that might be a more explicit and obvious solution to the current problem of the parameter names magically
	// showing up in the environment); we would just need something to handle the lookup, e.g. a custom function
	// like `externalIP "myservice"` that evaluates prior to job scheduling.

	if ingress == nil || ingress.URL == "" {
		return "", nil
	}
	u := ingress.URL

	// This sanity check prevents `http://svc` from being used by implementations that are outside the cluster
	if external && !strings.Contains(u, ".") {
		return "", fmt.Errorf("host must be fully qualified: %s", u)
	}

	return u, nil
}

func TrialJobImage(job string) string {
	// Allow the image name to be overridden using environment variables, primarily for development work
	imageName := os.Getenv("OPTIMIZE_TRIALS_IMAGE_REPOSITORY")
	if imageName == "" {
		imageName = "thestormforge/optimize-trials"
	}
	imageTag := os.Getenv("OPTIMIZE_TRIALS_IMAGE_TAG")
	if imageTag == "" {
		imageTagBase := os.Getenv("OPTIMIZE_TRIALS_IMAGE_TAG_BASE")
		if imageTagBase == "" {
			imageTagBase = "v0.0.1"
		}
		imageTag = imageTagBase + "-" + job
	}
	return imageName + ":" + imageTag
}

// NewObjectiveMetric creates a new metric for the supplied objective with most fields pre-filled.
func NewObjectiveMetric(obj *redskyappsv1alpha1.Objective, query string) redskyv1beta1.Metric {
	defer func() { obj.Implemented = true }()
	return redskyv1beta1.Metric{
		Type:     redskyv1beta1.MetricPrometheus,
		Query:    query,
		Minimize: true,
		Name:     obj.Name,
		Min:      obj.Min,
		Max:      obj.Max,
		Optimize: obj.Optimize,
	}
}
