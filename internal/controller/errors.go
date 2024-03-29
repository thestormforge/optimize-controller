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
	"errors"

	"github.com/thestormforge/optimize-go/pkg/api"
	applications "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
	experiments "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
)

// IgnoreNotFound returns the supplied error, unless that error is a "not found" error
func IgnoreNotFound(err error) error {
	if apierrs.IsNotFound(err) {
		return nil
	}

	var apierr *api.Error
	if errors.As(err, &apierr) {
		switch apierr.Type {
		case experiments.ErrExperimentNotFound,
			experiments.ErrTrialNotFound,
			applications.ErrApplicationNotFound,
			applications.ErrScenarioNotFound:
			return nil
		}
	}

	return err
}

// IgnoreAlreadyExists returns the supplied error, unless that error is an "already exists" error
func IgnoreAlreadyExists(err error) error {
	if apierrs.IsAlreadyExists(err) {
		return nil
	}
	return err
}

// IgnoreReportError returns the supplied error, unless the error is ignorable when reporting trials
func IgnoreReportError(err error) error {
	if IgnoreNotFound(err) == nil {
		return nil
	}

	var apierr *api.Error
	if errors.As(err, &apierr) {
		switch apierr.Type {
		case experiments.ErrTrialAlreadyReported:
			return nil
		}
	}
	
	return err
}
