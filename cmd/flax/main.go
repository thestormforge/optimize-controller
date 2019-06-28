package main

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"math/rand"
	"time"

	client "github.com/gramLabs/redsky/pkg/api"
	redsky "github.com/gramLabs/redsky/pkg/api/redsky/v1alpha1"
)

func init() {
	// https://stackoverflow.com/a/54491783
	var b [8]byte
	_, err := crypto_rand.Read(b[:])
	if err != nil {
		panic(err)
	}
	rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))
}

func main() {
	address := flag.String("addr", "", "The Flax URL.")
	flag.Parse()

	cfg, err := client.DefaultConfig()
	if err != nil {
		panic(err)
	}

	// Only override the address if it is explicitly passed in as an argument
	if *address != "" {
		cfg.Address = *address
	}
	if cfg.Address == "" {
		cfg.Address = "http://localhost:8000/api"
	}

	// New client
	c, err := client.NewClient(*cfg)
	if err != nil {
		panic(err)
	}
	api := redsky.NewApi(c)

	// New experiment
	in := &redsky.Experiment{
		Parameters: []redsky.Parameter{
			{
				Name: "a",
				Type: redsky.ParameterTypeInteger,
				Bounds: redsky.Bounds{
					Min: "1",
					Max: "5",
				},
			},
			{
				Name: "b",
				Type: redsky.ParameterTypeDouble,
				Bounds: redsky.Bounds{
					Min: "-1.0",
					Max: "1.0",
				},
			},
		},
		Metrics: []redsky.Metric{
			{
				Name: "l",
			},
			{
				Name:     "m",
				Minimize: true,
			},
		},
	}

	// Try a few random names
	var exp redsky.Experiment
	for i := 0; i < 10; i++ {
		name := redsky.NewExperimentName(GetRandomName(i))
		exp, err = api.CreateExperiment(context.TODO(), name, *in)
		if err != nil {
			if aerr, ok := err.(*redsky.Error); ok && aerr.Type == redsky.ErrExperimentNameConflict {
				continue
			}
		}
		break
	}
	if err != nil {
		panic(err)
	}

	// Check the self link
	if exp.Self == "" {
		panic("Missing self link")
	}

	// Delete the experiment when we are done
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
	pi := make(map[string]redsky.Parameter, len(in.Parameters))
	for _, p := range in.Parameters {
		pi[p.Name] = p
	}
	mi := make(map[string]redsky.Metric, len(in.Metrics))
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

	// Get the next trial assignments
	var rt string
	for i := 0; i < 5; i++ {
		_, rt, err = api.NextTrial(context.TODO(), exp.NextTrial)
		if aerr, ok := err.(*redsky.Error); ok && aerr.Type == redsky.ErrTrialUnavailable {
			time.Sleep(aerr.RetryAfter)
			continue
		}
		break
	}
	if err != nil {
		panic(err)
	}

	// Check the report trial URL
	if rt == "" {
		panic("Missing reportTrial link")
	}

	// Report an observation back
	v := &redsky.TrialValues{
		Values: []redsky.Value{
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
	err = api.ReportTrial(context.TODO(), rt, *v)
	if err != nil {
		panic(err)
	}

	fmt.Println("Much Success!")
}

var (
	left = [...]string{
		"agitated",
		"awesome",
		"bold",
		"cranky",
		"determined",
		"elated",
		"epic",
		"frosty",
		"happy",
		"jolly",
		"nostalgic",
		"quirky",
		"thirsty",
		"vigilant",
	}

	right = [...]string{
		"fenwick",
		"gustie",
		"hochadel",
		"idan",
		"joyce",
		"pacheco",
		"perol",
		"platt",
		"provo",
		"sich",
		"sutherland",
		"zhang",
	}
)

func GetRandomName(retry int) string {
	name := fmt.Sprintf("%s_%s", left[rand.Intn(len(left))], right[rand.Intn(len(right))])

	if retry > 0 {
		name = fmt.Sprintf("%s%d", name, rand.Intn(10))
	}
	return name
}
