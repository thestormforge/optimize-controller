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
package check

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	redsky "github.com/redskyops/k8s-experiment/pkg/api/redsky/v1alpha1"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	checkServerLong    = `Check the Red Sky Ops server`
	checkServerExample = ``
)

type ServerCheckOptions struct {
	Name           string
	ParameterCount int
	MetricCount    int
	AllowInvalid   bool
	ReportFailure  bool
	DryRun         bool

	RedSkyAPI redsky.API

	cmdutil.IOStreams
}

func NewServerCheckOptions(ioStreams cmdutil.IOStreams) *ServerCheckOptions {
	return &ServerCheckOptions{
		IOStreams: ioStreams,
	}
}

func NewServerCheckCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewServerCheckOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "server",
		Short:   "Check the server",
		Long:    checkServerLong,
		Example: checkServerExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Generate experiment JSON to stdout.")

	return cmd
}

func (o *ServerCheckOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	// Randomly assign parameter and metric counts if they are not provided
	if o.ParameterCount == 0 {
		o.ParameterCount = rand.Intn(20) + 1
	}
	if o.MetricCount == 0 {
		o.MetricCount = rand.Intn(2) + 1
	}

	if !o.DryRun {
		var err error
		o.RedSkyAPI, err = f.RedSkyAPI()
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *ServerCheckOptions) Validate() error {
	if !o.AllowInvalid {
		if o.ParameterCount < 1 || o.ParameterCount > 20 {
			return fmt.Errorf("invalid parameter count: %d (should be [1,20])", o.ParameterCount)
		}
		if o.MetricCount < 1 || o.MetricCount > 2 {
			return fmt.Errorf("invalid metric count: %d (should be [1,2]", o.MetricCount)
		}
	}
	return nil
}

func (o *ServerCheckOptions) Run() error {
	var err error

	// Generate an experiment
	n := o.Name
	e := generateExperiment(o)
	if o.DryRun {
		if n == "" {
			n = GetRandomName(0)
		}
		e.DisplayName = n
		b, err := json.MarshalIndent(e, "", "    ")
		if err != nil {
			return err
		}
		_, err = o.Out.Write(b)
		return err
	}

	// Create the experiment
	var exp redsky.Experiment
	if n != "" {
		exp, err = o.RedSkyAPI.CreateExperiment(context.TODO(), redsky.NewExperimentName(n), *e)
	} else {
		// If we are generating the name randomly, account for a small number of conflicts
		for i := 0; i < 10; i++ {
			n = GetRandomName(i)
			exp, err = o.RedSkyAPI.CreateExperiment(context.TODO(), redsky.NewExperimentName(n), *e)
			if err != nil {
				if aerr, ok := err.(*redsky.Error); ok && aerr.Type == redsky.ErrExperimentNameConflict {
					continue
				}
			}
			break
		}
	}
	if err != nil {
		return err
	}
	defer func() {
		_ = o.RedSkyAPI.DeleteExperiment(context.TODO(), exp.Self)
	}()

	// Validate the experiment
	if err = checkExperiment(n, e, &exp); err != nil {
		return err
	}

	// Get the next trial assignments
	var t redsky.TrialAssignments
	for i := 0; i < 5; i++ {
		t, err = o.RedSkyAPI.NextTrial(context.TODO(), exp.NextTrial)
		if aerr, ok := err.(*redsky.Error); ok && aerr.Type == redsky.ErrTrialUnavailable {
			time.Sleep(aerr.RetryAfter)
			continue
		}
		break
	}
	if err != nil {
		return err
	}

	// Validate the trial assignments
	if err = checkTrialAssignments(&exp, &t); err != nil {
		return err
	}

	// Report a trial observation back
	v := generateObservation(o, &exp)
	err = o.RedSkyAPI.ReportTrial(context.TODO(), t.ReportTrial, *v)
	if err != nil {
		return err
	}

	// Much success!
	return nil
}

// Generates an experiment
func generateExperiment(o *ServerCheckOptions) *redsky.Experiment {
	e := &redsky.Experiment{}

	// TODO Optimization?

	used := make(map[string]bool, o.ParameterCount+o.MetricCount)

	for i := 0; i < o.ParameterCount; i++ {
		e.Parameters = append(e.Parameters, redsky.Parameter{
			Name:   getUnique(used, GetRandomParameter),
			Type:   redsky.ParameterTypeInteger,
			Bounds: *generateBounds(),
		})
	}

	for i := 0; i < o.MetricCount; i++ {
		e.Metrics = append(e.Metrics, redsky.Metric{
			Name:     getUnique(used, GetRandomMetric),
			Minimize: generateMinimize(),
		})
	}

	return e
}

func generateObservation(o *ServerCheckOptions, exp *redsky.Experiment) *redsky.TrialValues {
	vals := &redsky.TrialValues{}
	if o.ReportFailure {
		vals.Failed = true
	} else {
		for _, m := range exp.Metrics {
			v := redsky.Value{MetricName: m.Name}
			v.Value, v.Error = generateValue()
			vals.Values = append(vals.Values, v)
		}
	}
	return vals
}

func generateBounds() *redsky.Bounds {
	min, max := rand.Intn(100), rand.Intn(4000)
	if min > max {
		s := min
		min = max
		min = s
	}
	return &redsky.Bounds{
		Min: json.Number(strconv.Itoa(min)),
		Max: json.Number(strconv.Itoa(max)),
	}
}

func generateMinimize() bool {
	if rand.Intn(2) == 0 {
		return false
	} else {
		return true
	}
}

func generateValue() (float64, float64) {
	// TODO Should we send values greater then 1?
	// TODO Should we send an error?
	return rand.Float64(), 0
}

func checkExperiment(name string, original, created *redsky.Experiment) error {
	if created.Self == "" {
		return fmt.Errorf("server did not return a self link")
	}
	if created.NextTrial == "" {
		return fmt.Errorf("server did not return a next trial link")
	}
	if created.Trials == "" {
		return fmt.Errorf("server did not return a trials link")
	}

	// TODO Optimization

	if len(created.Parameters) != len(original.Parameters) {
		return fmt.Errorf("server returned a different number of parameters: %d (expected %d)", len(created.Parameters), len(original.Parameters))
	}
	params := make(map[string]*redsky.Parameter, len(original.Parameters))
	for i := range original.Parameters {
		params[original.Parameters[i].Name] = &original.Parameters[i]
	}
	for _, p := range created.Parameters {
		if op, ok := params[p.Name]; ok {
			if p.Bounds.Min != op.Bounds.Min || p.Bounds.Max != op.Bounds.Max {
				return fmt.Errorf("server returned parameter with incorrect bounds: %s [%s,%s] (expected [%s,%s])", p.Name, p.Bounds.Min, p.Bounds.Min, op.Bounds.Min, op.Bounds.Max)
			}
		} else {
			return fmt.Errorf("server returned unexpected parameter: %s", p.Name)
		}
	}

	if len(created.Metrics) != len(original.Metrics) {
		return fmt.Errorf("server returned a different number of metrics: %d (expected %d)", len(created.Metrics), len(original.Metrics))
	}
	metrics := make(map[string]*redsky.Metric, len(original.Metrics))
	for i := range original.Metrics {
		metrics[original.Metrics[i].Name] = &original.Metrics[i]
	}
	for _, m := range created.Metrics {
		if om, ok := metrics[m.Name]; ok {
			if m.Minimize != om.Minimize {
				return fmt.Errorf("server returned metric with incorrect minimization: %s [%t]", m.Name, m.Minimize)
			}
		} else {
			return fmt.Errorf("server returned unexpected metric: %s", m.Name)
		}
	}

	return nil
}

func checkTrialAssignments(exp *redsky.Experiment, t *redsky.TrialAssignments) error {
	if t.ReportTrial == "" {
		return fmt.Errorf("server did not return a report trial link")
	}

	if len(t.Assignments) != len(exp.Parameters) {
		return fmt.Errorf("server returned a different number of parameters: %d (expected %d)", len(t.Assignments), len(exp.Parameters))
	}
	params := make(map[string]*redsky.Parameter, len(exp.Parameters))
	for i := range exp.Parameters {
		params[exp.Parameters[i].Name] = &exp.Parameters[i]
	}
	for _, a := range t.Assignments {
		if p, ok := params[a.ParameterName]; ok {
			// Check bounds using floating point arithmetic
			v, err := a.Value.Float64()
			if err != nil {
				return err
			}
			min, err := p.Bounds.Min.Float64()
			if err != nil {
				return err
			}
			max, err := p.Bounds.Max.Float64()
			if err != nil {
				return err
			}
			if v < min || v > max {
				return fmt.Errorf("server return out of bounds assignment: %s = %s (expected [%s,%s])", a.ParameterName, a.Value, p.Bounds.Min, p.Bounds.Max)
			}
		} else {
			return fmt.Errorf("server returned unexpected assignment: %s", a.ParameterName)
		}
	}

	return nil
}
