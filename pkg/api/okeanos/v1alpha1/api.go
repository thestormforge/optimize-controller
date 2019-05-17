package v1alpha1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gramLabs/okeanos/pkg/api"
)

const (
	endpointExperiment = "/api/experiment"
)

// ErrorType enumerates the possible API specific error conditions
type ErrorType string

const (
	// ErrExperimentStopped indicates that the experiment is over and no more suggestions will be provided
	ErrExperimentStopped ErrorType = "stopped"
	// ErrSuggestionUnavailable indicates that no suggestions are currently available
	ErrSuggestionUnavailable ErrorType = "unavailable"
)

// Error represents the API specific error messages and may be used in response to HTTP status codes
type Error struct {
	Type ErrorType
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: ", e.Type)
}

// Optimization controls how the optimizer will generate suggestions
type Optimization struct {
	// The estimated number of trial runs to perform for an experiment
	ObservationBudget int32 `json:"observation_budget,omitempty"`
	// The total number of concurrent trial runs supported for an experiment
	Parallelism int32 `json:"parallelism,omitempty"`
}

// Bounds are used to define the domain for a parameter
type Bounds struct {
	// The minimum value for a numeric parameter
	Min json.Number `json:"min,omitempty"`
	// The maximum value for a numeric parameter
	Max json.Number `json:"max,omitempty"`
	// The possible values for string parameter
	Values []string `json:"values,omitempty"`
}

// Parameter is a variable that is going to be tuned in an experiment.
type Parameter struct {
	// The name of the parameter
	Name string `json:"name"`
	// TODO Do we need type? What are the values? We need an enum if we keep it.
	Type string `json:"type"` // int, double
	// The domain of the parameter
	Bounds Bounds `json:"bounds"`
}

// Metric is a target that we are trying to optimize.
type Metric struct {
	// The name of the parameter
	Name string `json:"name"`
	// The flag indicating this metric should be minimized
	Minimize bool `json:"minimize,omitempty"`
}

// Experiment combines the search space, outcomes and optimization configuration
type Experiment struct {
	// The optimization configuration for the experiment
	Optimization Optimization `json:"optimization,omitempty"`
	// The search space of the experiment
	Parameters []Parameter `json:"parameters"`
	// The outcomes of the experiment
	Metrics []Metric `json:"metrics"`
	// The absolute URL used to obtain suggestions via a POST request
	SuggestionRef string `json:"suggestionRef"`
}

// Suggestion represents the assignments of parameter values for a trial run
type Suggestion struct {
	// The mapping of parameter names to their assigned value
	Values map[string]interface{} `json:"values"`
}

// Observation represents the outcome of a trial run
type Observation struct {
	// The time at which the trial run began
	Start *time.Time `json:"start,omitempty"`
	// The time at which the trial run concluded
	End *time.Time `json:"end,omitempty"`
	// Flag indicating if the suggestion resulted in a failed state
	Failed bool `json:"failed,omitempty"`
	// The mapping of metric names to their observed value
	Metrics map[string]interface{} `json:"metrics"`
}

// API provides bindings for the Flax endpoints
type API interface {
	// Creates or updates an experiment with the specified name
	PutExperiment(context.Context, string, Experiment) error
	// Retrieves the experiment with the specified name
	GetExperiment(context.Context, string) (Experiment, error)
	// Obtains the next suggestion from a suggestion reference
	NextSuggestion(context.Context, string) (Suggestion, string, error)
	// Reports an observation for a suggestion reference
	ReportObservation(context.Context, string, Observation) error
}

// NewApi returns a new version specific API for the specified client
func NewApi(c api.Client) API {
	return &httpAPI{client: c}
}

type httpAPI struct {
	client api.Client
}

func (h *httpAPI) PutExperiment(ctx context.Context, n string, exp Experiment) error {
	u := h.client.URL(endpointExperiment + "/" + url.PathEscape(n))

	body, err := json.Marshal(exp)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	_, _, err = h.client.Do(ctx, req)
	return err
}

func (h *httpAPI) GetExperiment(ctx context.Context, n string) (Experiment, error) {
	u := h.client.URL(endpointExperiment + "/" + url.PathEscape(n))
	e := Experiment{}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return e, err
	}

	_, body, err := h.client.Do(ctx, req)

	err = json.Unmarshal(body, &e)
	return e, err
}

func (h *httpAPI) NextSuggestion(ctx context.Context, u string) (Suggestion, string, error) {
	s := Suggestion{}
	l := ""

	req, err := http.NewRequest(http.MethodPost, u, nil)
	if err != nil {
		return s, l, err
	}

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return s, l, err
	}

	switch resp.StatusCode {
	case http.StatusGone:
		return s, l, &Error{Type: ErrExperimentStopped}
	case http.StatusServiceUnavailable:
		// TODO Get the expected timeout from the error response
		return s, l, &Error{Type: ErrSuggestionUnavailable}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		l = resp.Header.Get("Location")
		err = json.Unmarshal(body, &s)
	}
	return s, l, err
}

func (h *httpAPI) ReportObservation(ctx context.Context, u string, obs Observation) error {
	body, err := json.Marshal(obs)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, u, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	_, _, err = h.client.Do(ctx, req)
	return err
}
