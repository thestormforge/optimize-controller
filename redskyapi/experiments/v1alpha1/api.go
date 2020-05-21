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

package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"
)

const (
	endpointExperiment = "/experiments/"

	relationSelf      = "self"
	relationNext      = "next"
	relationPrev      = "prev"
	relationPrevious  = "previous"
	relationLabels    = "https://carbonrelay.com/rel/labels"
	relationTrials    = "https://carbonrelay.com/rel/trials"
	relationNextTrial = "https://carbonrelay.com/rel/nextTrial"
)

// Meta is used to collect resource metadata from the response
type Meta interface {
	SetLocation(string)
	SetLastModified(time.Time)
	SetLink(rel, link string)
}

// Metadata is used to hold single or multi-value metadata from list responses
type Metadata map[string][]string

func (m *Metadata) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if *m == nil {
		*m = make(map[string][]string, len(raw))
	}
	for k, v := range raw {
		switch t := v.(type) {
		case string:
			(*m)[k] = append((*m)[k], t)
		case []interface{}:
			for i := range t {
				(*m)[k] = append((*m)[k], fmt.Sprintf("%s", t[i]))
			}
		}
	}
	return nil
}

type ErrorType string

const (
	ErrExperimentNameInvalid  ErrorType = "experiment-name-invalid"
	ErrExperimentNameConflict ErrorType = "experiment-name-conflict"
	ErrExperimentInvalid      ErrorType = "experiment-invalid"
	ErrExperimentNotFound     ErrorType = "experiment-not-found"
	ErrExperimentStopped      ErrorType = "experiment-stopped"
	ErrTrialInvalid           ErrorType = "trial-invalid"
	ErrTrialUnavailable       ErrorType = "trial-unavailable"
	ErrTrialNotFound          ErrorType = "trial-not-found"
	ErrTrialAlreadyReported   ErrorType = "trial-already-reported"
	ErrUnauthorized           ErrorType = "unauthorized"
	ErrUnexpected             ErrorType = "unexpected"
)

// Error represents the API specific error messages and may be used in response to HTTP status codes
type Error struct {
	Type       ErrorType     `json:"-"`
	Message    string        `json:"error"`
	RetryAfter time.Duration `json:"-"`
	Location   string        `json:"-"`
}

func (e *Error) Error() string {
	return e.Message
}

// IsUnauthorized check to see if the error is an "unauthorized" error
func IsUnauthorized(err error) bool {
	// OAuth errors (e.g. fetching tokens) will come out of `Do` and will be wrapped in url.Error
	if uerr, ok := err.(*url.Error); ok {
		err = uerr.Unwrap()
	}
	if rerr, ok := err.(*oauth2.RetrieveError); ok {
		if rerr.Response.StatusCode == http.StatusUnauthorized {
			return true
		}
	}
	if rserr, ok := err.(*Error); ok {
		if rserr.Type == ErrUnauthorized {
			return true
		}
	}
	// TODO This is a hack to work around the way we generate errors during JWT validation
	if err != nil && err.Error() == "no Bearer token" {
		return true
	}
	return false
}

type ServerMeta struct {
	Server string `json:"-"`
}

func (m *ServerMeta) Unmarshal(header http.Header) {
	m.Server = header.Get("Server")
}

// API provides bindings for the supported endpoints
type API interface {
	Options(context.Context) (ServerMeta, error)
	GetAllExperiments(context.Context, *ExperimentListQuery) (ExperimentList, error)
	GetAllExperimentsByPage(context.Context, string) (ExperimentList, error)
	GetExperimentByName(context.Context, ExperimentName) (Experiment, error)
	GetExperiment(context.Context, string) (Experiment, error)
	CreateExperiment(context.Context, ExperimentName, Experiment) (Experiment, error)
	DeleteExperiment(context.Context, string) error
	GetAllTrials(context.Context, string, *TrialListQuery) (TrialList, error)
	CreateTrial(context.Context, string, TrialAssignments) (string, error) // TODO Should this return TrialAssignments?
	NextTrial(context.Context, string) (TrialAssignments, error)
	ReportTrial(context.Context, string, TrialValues) error
	AbandonRunningTrial(context.Context, string) error
	LabelExperiment(context.Context, string, ExperimentLabels) error
	LabelTrial(context.Context, string, TrialLabels) error
}
