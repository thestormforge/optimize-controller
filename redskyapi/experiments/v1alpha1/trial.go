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
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type TrialMeta struct {
	SelfURL   string `json:"-"`
	LabelsURL string `json:"-"`
}

func (m *TrialMeta) SetLocation(location string) { m.SelfURL = location }
func (m *TrialMeta) SetLastModified(time.Time)   {}
func (m *TrialMeta) SetLink(rel, link string) {
	switch strings.ToLower(rel) {
	case relationLabels:
		m.LabelsURL = link
	}

	// Backwards compatibility with the old trial labels relation
	if m.LabelsURL == "" && strings.ToLower(rel) == "https://carbonrelay.com/rel/triallabels" {
		m.LabelsURL = link
	}
}

type Assignment struct {
	// The name of the parameter in the experiment the assignment corresponds to.
	ParameterName string `json:"parameterName"`
	// The assigned value of the parameter.
	Value json.Number `json:"value"`
}

type TrialAssignments struct {
	TrialMeta

	// The list of parameter names and their assigned values.
	Assignments []Assignment `json:"assignments"`
	// Labels for this trial.
	Labels map[string]string `json:"labels,omitempty"`
}

type Value struct {
	// The name of the metric in the experiment the value corresponds to.
	MetricName string `json:"metricName"`
	// The observed value of the metric.
	Value float64 `json:"value"`
	//The observed error of the metric.
	Error float64 `json:"error,omitempty"`
}

type TrialValues struct {
	// The observed values.
	Values []Value `json:"values,omitempty"`
	// Indicator that the trial failed, Values is ignored when true.
	Failed bool `json:"failed,omitempty"`
}

type TrialStatus string

const (
	TrialStaged    TrialStatus = "staged"
	TrialActive    TrialStatus = "active"
	TrialCompleted TrialStatus = "completed"
	TrialFailed    TrialStatus = "failed"
	TrialAbandoned TrialStatus = "abandoned"
)

type TrialItem struct {
	TrialAssignments
	TrialValues

	// The current trial status.
	Status TrialStatus `json:"status"`
	// Ordinal number indicating when during an experiment the trail was generated.
	Number int64 `json:"number"`
	// Labels for this trial.
	Labels map[string]string `json:"labels,omitempty"`

	// The metadata for an individual trial.
	Metadata Metadata `json:"_metadata,omitempty"`

	// Experiment is a reference back to the experiment this trial item is associated with. This field is never
	// populated by the API, but may be useful for consumers to maintain a connection between resources.
	Experiment *Experiment `json:"-"`
}

type TrialListQuery struct {
	// Comma separated list of statuses to fetch.
	Status []TrialStatus
	// Comma separated list of label value pairs to match on.
	LabelSelector map[string]string
}

func (p *TrialListQuery) Encode() string {
	if p == nil {
		return ""
	}
	q := url.Values{}
	if len(p.Status) > 0 {
		strs := make([]string, len(p.Status))
		for i := range p.Status {
			strs[i] = string(p.Status[i])
		}
		q.Add("status", strings.Join(strs, ","))
	}
	if len(p.LabelSelector) > 0 {
		ls := make([]string, 0, len(p.LabelSelector))
		for k, v := range p.LabelSelector {
			ls = append(ls, fmt.Sprintf("%s=%s", k, v))
		}
		q.Add("labelSelector", strings.Join(ls, ","))
	}
	return q.Encode()
}

type TrialList struct {
	// The list of trials.
	Trials []TrialItem `json:"trials"`

	// Experiment is a reference back to the experiment this trial item is associated with. This field is never
	// populated by the API, but may be useful for consumers to maintain a connection between resources.
	Experiment *Experiment `json:"-"`
}

type TrialLabels struct {
	// New labels for this trial.
	Labels map[string]string `json:"labels"`
}
