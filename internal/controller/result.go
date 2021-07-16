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

package controller

import (
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/thestormforge/optimize-go/pkg/api"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

// These methods return a pointer to a Result struct and an error; this is useful for implementing the
// pattern where Reconcile methods are split up and check for a non-nil Result to determine if it is
// time to return.

// RequeueIfUnavailable will return a new result and the supplied error, adjusted for trial unavailable errors
func RequeueIfUnavailable(err error) (*ctrl.Result, error) {
	result := &ctrl.Result{}
	if rse, ok := err.(*api.Error); ok && rse.Type == experimentsv1alpha1.ErrTrialUnavailable {
		err = nil
		result.RequeueAfter = rse.RetryAfter
		if result.RequeueAfter > 2*time.Minute {
			result.RequeueAfter = 2 * time.Minute
		} else if result.RequeueAfter < 5*time.Second {
			result.RequeueAfter = 5 * time.Second
		}
	}
	return result, err
}

// RequeueConflict will return a new result and the supplied error, adjusted for Kubernetes conflict errors
func RequeueConflict(err error) (*ctrl.Result, error) {
	return requeueConflictForFrameStack(err, getFrames())
}

func requeueConflictForFrameStack(err error, fs frameStack) (*ctrl.Result, error) {
	result := &ctrl.Result{}
	if apierrs.IsConflict(err) {
		controllerName := guessController(fs)
		ReconcileConflictErrors.WithLabelValues(controllerName).Inc()
		result.Requeue = true
		err = nil
	}
	return result, err
}

// guessController dumps stack to try and guess what the controller name should be
func guessController(frames frameStack) string {
	for {
		frame, more := frames.Next()
		if path.Base(path.Dir(frame.File)) == "controllers" {
			p := strings.SplitN(path.Base(frame.File), "_", 2)
			return p[0]
		}
		if !more {
			break
		}
	}
	return "controller"
}

type frameStack interface {
	Next() (runtime.Frame, bool)
}

func getFrames() frameStack {
	pc := make([]uintptr, 3)
	if n := runtime.Callers(3, pc); n > 0 {
		return runtime.CallersFrames(pc[:n])
	}
	return nil
}
