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
	address := flag.String("addr", "http://localhost:8000", "The Flax URL.")
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
	name := okeanos.NewExperimentName("test")
	exp := &okeanos.Experiment{
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
				Type: okeanos.ParameterTypeFloat,
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
			{
				Name:     "n",
				Minimize: false,
			},
		},
	}

	// Put it out there
	eu, err := api.PutExperiment(context.TODO(), name, *exp)
	if err != nil {
		panic(err)
	}

	// Delete the experiment
	defer func() {
		err = api.DeleteExperiment(context.TODO(), eu)
		if err != nil {
			panic(err)
		}
	}()

	// Get it back, overwrite pointer
	*exp, err = api.GetExperiment(context.TODO(), eu)
	if err != nil {
		panic(err)
	}

	// Check the URL
	if exp.SuggestionRef == "" {
		panic("Expected a 'suggestionRef' URL on the result to post suggestions back to")
	}

	// TODO We need to make some better assertions
	if exp.Metrics[0].Name == "l" && exp.Metrics[0].Minimize {
		panic("Expected an unspecified boolean (Metric.Minimize) to default to false")
	}

	// Get a suggestion
	var su string
	for i := 0; i < 5; i++ {
		_, su, err = api.NextSuggestion(context.TODO(), exp.SuggestionRef)
		if aerr, ok := err.(*okeanos.Error); ok && aerr.Type == okeanos.ErrSuggestionUnavailable {
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
	obs := &okeanos.Observation{
		Values: []okeanos.Value{
			{
				Name:  "l",
				Value: 0.99,
			},
			{
				Name:  "m",
				Value: 2.0,
			},
		},
	}
	err = api.ReportObservation(context.TODO(), su, *obs)
	if err != nil {
		panic(err)
	}

	fmt.Println("Much Success!")
}
