package v1alpha1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gramLabs/okeanos/pkg/api"
)

const (
	endpointExperiment = "/api/experiment"
)

// ParameterType enumerates the possible parameter types
type ParameterType string

// ErrorType enumerates the possible API specific error conditions
type ErrorType string

const (
	// ParameterTypeInteger indicates a parameter has an integer value
	ParameterTypeInteger ParameterType = "int"
	//ParameterTypeFloat indicates a parameter has a floating point value
	ParameterTypeFloat ParameterType = "float"

	// ErrExperimentStopped indicates that the experiment is over and no more suggestions will be provided
	ErrExperimentStopped ErrorType = "stopped"
	// ErrSuggestionUnavailable indicates that no suggestions are currently available
	ErrSuggestionUnavailable ErrorType = "unavailable"
)

// ExperimentName exists to clearly separate cases where an actual name can be used
type ExperimentName interface {
	Name() string
}

func NewExperimentName(n string) ExperimentName {
	return experimentName{name: n}
}

type experimentName struct {
	name string
}

func (n experimentName) Name() string {
	return n.name
}

// Error represents the API specific error messages and may be used in response to HTTP status codes
type Error struct {
	Type       ErrorType
	RetryAfter time.Duration
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
}

// Parameter is a variable that is going to be tuned in an experiment
type Parameter struct {
	// The name of the parameter
	Name string `json:"name"`
	// The type of the parameter
	Type ParameterType `json:"type"`
	// The default value of the parameter
	Default interface{} `json:"default,omitempty"`
	// The domain of the parameter
	Bounds Bounds `json:"bounds"`
	// The possible values for string parameter
	Values []string `json:"values,omitempty"`
}

// Metric is a target that we are trying to optimize
type Metric struct {
	// The name of the parameter
	Name string `json:"name"`
	// The flag indicating this metric should be minimized
	Minimize bool `json:"minimize,omitempty"`
}

// Value is a recorded metric value
type Value struct {
	// The name of the value, must correspond to a metric name on the experiment
	Name string `json:"name"`
	// The observed value of the metric
	Value float64 `json:"value"`
	// The observed error of the metric
	Error float64 `json:"error"`
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
	SuggestionRef string `json:"suggestionRef,omitempty"`
	// The absolute URL used to fetch the entire list of observations
	ObservationRef string `json:"observationRef,omitempty"`
}

// Suggestion represents the assignments of parameter values for a trial run
type Suggestion struct {
	// The mapping of parameter names to their assigned value
	// TODO Should this be a list of `Assignment` instances?
	Assignments map[string]interface{} `json:"assignments"`
}

// Observation represents the outcome of a trial run
type Observation struct {
	// Flag indicating if the suggestion resulted in a failed state
	Failed bool `json:"failed,omitempty"`
	// The observed values
	Values []Value `json:"values"`
}

// A list of observations
type ObservationList struct {
	// The actual list of observations
	// TODO The observation returned here is different (it has labels and assignments but no "failed")
	Observations []Observation `json:"observations"`
}

// API provides bindings for the Flax endpoints
type API interface {
	// Creates or updates an experiment with the specified name and returns the URL
	PutExperiment(context.Context, ExperimentName, Experiment) (string, error)
	// Retrieves the experiment with the specified URL
	GetExperiment(context.Context, string) (Experiment, error)
	// Deletes the experiment with the specified URL
	DeleteExperiment(context.Context, string) error
	// Manually creates a new suggestion
	CreateSuggestion(context.Context, string, Suggestion) (string, error)
	// Obtains the next suggestion from a suggestion reference
	NextSuggestion(context.Context, string) (Suggestion, string, error)
	// Reports an observation for a suggestion reference
	ReportObservation(context.Context, string, Observation) error
	// Gets a list of all observations
	GetObservations(context.Context, string) (ObservationList, error)
}

// NewApi returns a new version specific API for the specified client
func NewApi(c api.Client) API {
	return &httpAPI{client: c}
}

type httpAPI struct {
	client api.Client
}

func (h *httpAPI) PutExperiment(ctx context.Context, n ExperimentName, exp Experiment) (string, error) {
	u := h.client.URL(endpointExperiment + "/" + url.PathEscape(n.Name()))

	body, err := json.Marshal(exp)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	_, _, err = h.client.Do(ctx, req)
	return u.String(), err
}

func (h *httpAPI) GetExperiment(ctx context.Context, u string) (Experiment, error) {
	e := Experiment{}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return e, err
	}

	_, body, err := h.client.Do(ctx, req)

	err = json.Unmarshal(body, &e)
	return e, err
}

func (h *httpAPI) DeleteExperiment(ctx context.Context, u string) error {
	req, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return err
	}

	_, _, err = h.client.Do(ctx, req)
	return err
}

func (h *httpAPI) CreateSuggestion(context.Context, string, Suggestion) (string, error) {
	return "", nil
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
		ra, err := strconv.Atoi(resp.Header.Get("Retry-After"))
		if err != nil {
			ra = 5
		}
		return s, l, &Error{Type: ErrSuggestionUnavailable, RetryAfter: time.Duration(ra) * time.Second}
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

func (h *httpAPI) GetObservations(ctx context.Context, u string) (ObservationList, error) {
	l := ObservationList{}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return l, err
	}

	_, body, err := h.client.Do(ctx, req)

	err = json.Unmarshal(body, &l)
	return l, err
}
