package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/gramLabs/okeanos/pkg/api"
	okeanos "github.com/gramLabs/okeanos/pkg/api/okeanos/v1alpha1"
)

func main() {
	address := flag.String("addr", "http://localhost:8000/api", "The Flax URL.")
	flag.Parse()

	// New client
	c, err := api.NewClient(api.Config{
		Address: *address,
	})
	if err != nil {
		panic(err)
	}
	api := okeanos.NewApi(c)

	// New experiment
	name := okeanos.NewExperimentName("this-is-not-a-test")
	in := &okeanos.Experiment{
		Parameters: []okeanos.Parameter{
			{
				Name: "a",
				Type: okeanos.ParameterTypeInteger,
				Bounds: okeanos.Bounds{
					Min: "1",
					Max: "5",
				},
			},
			{
				Name: "b",
				Type: okeanos.ParameterTypeDouble,
				Bounds: okeanos.Bounds{
					Min: "-1.0",
					Max: "1.0",
				},
			},
			//{
			//	Name:   "c",
			//  Type: okeanos.ParameterTypeString,
			//	Values: []string{"x", "y", "z"},
			//},
		},
		Metrics: []okeanos.Metric{
			{
				Name: "l",
			},
			{
				Name:     "m",
				Minimize: true,
			},
		},
	}

	// Put it out there
	exp, err := api.CreateExperiment(context.TODO(), name, *in)
	if err != nil {
		panic(err)
	}

	// Check the self link
	if exp.Self == "" {
		panic("Missing self link")
	}

	// Delete the experiment
	defer func() {
		err = api.DeleteExperiment(context.TODO(), exp.Self)
		if err != nil {
			panic(err)
		}
	}()

	// Check the rest of the links
	if exp.Trials == "" {
		panic("Missing trials link")
	}
	if exp.NextTrial == "" {
		panic("Missing nextTrial link")
	}

	// Index the parameters and metrics for easy lookup
	pi := make(map[string]okeanos.Parameter, len(in.Parameters))
	for _, p := range in.Parameters {
		pi[p.Name] = p
	}
	mi := make(map[string]okeanos.Metric, len(in.Metrics))
	for _, m := range in.Metrics {
		mi[m.Name] = m
	}

	// Check the parameters
	for _, p := range exp.Parameters {
		if pp, ok := pi[p.Name]; ok {
			if p.Type != pp.Type {
				panic(fmt.Sprintf("Parameter type mismatch for '%s': was '%s', expected '%s'", p.Name, p.Type, pp.Type))
			}
			// TODO Check bounds, etc.
		} else {
			panic(fmt.Sprintf("Missing parameter '%s' from server", p.Name))
		}
	}

	// Check the metrics
	for _, m := range exp.Metrics {
		if mm, ok := mi[m.Name]; ok {
			if m.Minimize != mm.Minimize {
				panic(fmt.Sprintf("Metric direction mismatch for '%s': was '%t', expected '%t'", m.Name, m.Minimize, mm.Minimize))
			}
		} else {
			panic(fmt.Sprintf("Missing metric '%s' from server", m.Name))
		}
	}

	// Get a suggestion
	var su string
	for i := 0; i < 5; i++ {
		_, su, err = api.NextTrial(context.TODO(), exp.NextTrial)
		if aerr, ok := err.(*okeanos.Error); ok && aerr.Type == okeanos.ErrTrialUnavailable {
			time.Sleep(aerr.RetryAfter)
			continue
		}
		break
	}
	if err != nil {
		panic(err)
	}

	// Check the URL
	if su == "" {
		panic("Expected POST to suggestion to return a Location header to post observations back to")
	}

	// Report an observation back
	obs := &okeanos.TrialValues{
		Values: []okeanos.Value{
			{
				MetricName: "l",
				Value:      0.99,
			},
			{
				MetricName: "m",
				Value:      2.0,
			},
		},
	}
	err = api.ReportTrial(context.TODO(), su, *obs)
	if err != nil {
		panic(err)
	}

	fmt.Println("Much Success!")
}
