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
package util

import (
	redskyapi "github.com/redskyops/k8s-experiment/pkg/api/redsky/v1alpha1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

// IgnoreNotFound returns a default result and the supplied error, unless that error is a "not found" error
func IgnoreNotFound(err error) (ctrl.Result, error) {
	if apierrs.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, err
}

// IgnoreConflict returns a default result and the supplied error, unless that error is a "conflict" in which case a requeue is requested
func IgnoreConflict(err error) (ctrl.Result, error) {
	if apierrs.IsConflict(err) {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, err
}

// IgnoreTrialUnavailable returns a default result and the supplied error, unless that error is a "trial unavailable" in which case a delayed requeue is requested
func IgnoreTrialUnavailable(err error) (ctrl.Result, error) {
	if rse, ok := err.(*redskyapi.Error); ok && rse.Type == redskyapi.ErrTrialUnavailable {
		return ctrl.Result{RequeueAfter: rse.RetryAfter}, nil
	}
	return ctrl.Result{}, err
}
