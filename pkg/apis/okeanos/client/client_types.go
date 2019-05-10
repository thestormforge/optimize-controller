package client

import (
	"encoding/json"
	"time"
)

// Configuration controls how suggestions are generated
type Configuration struct {
	Budget      int32 `json:"budget,omitempty"`
	Parallelism int32 `json:"parallelism,omitempty"`
}

// Parameter is a variable that is going to be tuned in an experiment.
type Parameter struct {
	Name   string      `json:"name"`
	Values []string    `json:"values,omitempty"`
	Min    json.Number `json:"min,omitempty"`
	Max    json.Number `json:"max,omitempty"`
}

// Metric is a target that we are trying to optimize.
type Metric struct {
	Name     string `json:"name"`
	Minimize bool   `json:"minimize,omitempty"`
}

// Experiment combines the search space, outcomes and optimization configuration
type Experiment struct {
	Configuration Configuration `json:"configuration,omitempty"`
	Parameters    []Parameter   `json:"parameters"`
	Metrics       []Metric      `json:"metrics"`
	SuggestionRef string        `json:"suggestionRef"`
}

// Suggestion represents the assignments of parameter values for a trial run
type Suggestion struct {
	Values map[string]interface{} `json:"values"`
}

// Observation represents the outcome of a trial run
type Observation struct {
	Start   *time.Time             `json:"start,omitempty"`
	End     *time.Time             `json:"end,omitempty"`
	Failed  bool                   `json:"failed,omitempty"`
	Metrics map[string]interface{} `json:"metrics"`
}
