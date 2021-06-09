/*
Copyright 2021 GramLabs, Inc.

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

package debug

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/internal/experiment"
	"github.com/thestormforge/optimize-controller/v2/internal/template"
	"github.com/thestormforge/optimize-go/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// MetricQueryOptions configure a metric query debugging session.
type MetricQueryOptions struct {
	Config *config.OptimizeConfig
	commander.IOStreams

	Filename       string
	TrialName      string
	MetricName     string
	StartTime      string
	Duration       string
	CompletionTime string
}

// NewMetricQueryCommand create
func NewMetricQueryCommand(o *MetricQueryOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "metric",
		Short:  "Debug metric queries",
		Long:   "Render metric queries using specified values",
		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithoutArgsE(o.Debug),
	}

	cmd.Flags().StringVarP(&o.Filename, "filename", "f", "", "`file` containing the experiment definition")
	cmd.Flags().StringVar(&o.TrialName, "trial", "", "trial `name` to use")
	cmd.Flags().StringVar(&o.MetricName, "metric", "", "metric `name` to print or empty for all metrics")
	cmd.Flags().StringVar(&o.StartTime, "start", "", "trial start `time`")
	cmd.Flags().StringVar(&o.Duration, "duration", "", "trial `duration` (instead of completion)")
	cmd.Flags().StringVar(&o.CompletionTime, "completion", "", "trial end `time`")

	_ = cmd.MarkFlagFilename("filename", "yml", "yaml")
	_ = cmd.MarkFlagRequired("filename")

	return cmd
}

func (o *MetricQueryOptions) Debug() error {
	// Read the experiment
	r, err := o.IOStreams.OpenFile(o.Filename)
	if err != nil {
		return err
	}

	exp := &optimizev1beta2.Experiment{}
	rr := commander.NewResourceReader()
	if err := rr.ReadInto(r, exp); err != nil {
		return err
	}

	// Create a new trial object
	t := &optimizev1beta2.Trial{}
	t.Name = o.TrialName

	// Fill out information from the experiment
	experiment.PopulateTrialFromTemplate(exp, t)
	if err := o.populateTrialTime(exp, t); err != nil {
		return err
	}
	if t.Namespace == "" {
		t.Namespace = "default"
	}
	if t.Name == "" {
		t.Name = t.GenerateName + "0"
	}

	// Create a new Prometheus test
	pt := newPromtest()
	if err := pt.addTest(t); err != nil {
		return err
	}

	eng := template.New()
	for i := range exp.Spec.Metrics {
		m := &exp.Spec.Metrics[i]
		if o.MetricName != "" && m.Name != o.MetricName {
			continue
		}

		// Dummy out the target object
		target := &unstructured.Unstructured{}
		if m.Target != nil {
			target.SetGroupVersionKind(m.Target.GroupVersionKind())
		}

		// Render the query (record errors as a comment)
		q, _, err := eng.RenderMetricQueries(m, t, target)
		if err != nil {
			pt.addErrorComment(m, err)
			continue
		}

		// Not Prometheus, just add a comment to the header
		if m.Type != optimizev1beta2.MetricPrometheus {
			pt.addQueryComment(m, q)
			continue
		}

		// Add the PromQL to the test
		if err := pt.addPromqlExprTest(t, m, q); err != nil {
			return err
		}
	}

	// Write the Prometheus test out
	return pt.writeTo(o.YAMLWriter())
}

func (o *MetricQueryOptions) populateTrialTime(exp *optimizev1beta2.Experiment, t *optimizev1beta2.Trial) error {
	startTime, err := parseTime(o.StartTime, time.Now())
	if err != nil {
		return err
	}

	trialDuration, err := parseDuration(o.Duration, exp.Spec.TrialTemplate.Spec.ApproximateRuntime)
	if err != nil {
		return err
	}

	completionTime, err := parseTime(o.CompletionTime, startTime.Add(trialDuration))
	if err != nil {
		return err
	}

	t.Status.StartTime = &startTime
	t.Status.CompletionTime = &completionTime
	return nil
}

func parseTime(input string, defaultTime time.Time) (metav1.Time, error) {
	if input == "" {
		return metav1.NewTime(defaultTime), nil
	}

	// Look for Unix time (either `<sec-int64>` or `<sec-int64>.<nsec-int64>`)
	ss := regexp.MustCompile(`^([0-9]+)(?:\.([0-9]+))?$`).FindStringSubmatch(input)
	if len(ss) == 3 {
		sec, err := strconv.ParseInt(ss[1], 10, 64)
		if err != nil {
			return metav1.Time{}, err
		}
		nsec, err := strconv.ParseInt(ss[2], 10, 64)
		if ss[2] != "" && err != nil {
			return metav1.Time{}, err
		}
		return metav1.NewTime(time.Unix(sec, nsec)), nil
	}

	// Fall back to RFC 3339 time
	t, err := time.Parse(time.RFC3339Nano, input)
	return metav1.NewTime(t), err
}

func parseDuration(input string, defaultDuration *metav1.Duration) (time.Duration, error) {
	if input == "" {
		if defaultDuration != nil {
			return defaultDuration.Duration, nil
		}
		return 5 * time.Second, nil
	}

	return time.ParseDuration(input)
}

type promTest struct {
	document *yaml.RNode
}

func newPromtest() *promTest {
	return &promTest{
		document: yaml.NewRNode(&yaml.Node{
			Kind: yaml.DocumentNode,
			Content: []*yaml.Node{
				{
					Kind: yaml.MappingNode,
					Content: []*yaml.Node{
						{Kind: yaml.ScalarNode, Value: "tests"},
						{Kind: yaml.SequenceNode},
					},
				},
			},
		}),
	}
}

func (p *promTest) writeTo(w kio.Writer) error {
	return w.Write([]*yaml.RNode{p.document})
}

func (p *promTest) addHeadComment(pattern string, args ...interface{}) {
	prefix := p.document.YNode().HeadComment
	if prefix != "" {
		prefix += "\n"
	}

	p.document.YNode().HeadComment = prefix + fmt.Sprintf(pattern, args...) + "\n"
}

func (p *promTest) addTest(t *optimizev1beta2.Trial) error {
	seriesName := fmt.Sprintf(`up{job="prometheus-pushgateway", instance="optimize-%s-prometheus:9090"}`, t.Namespace)
	seriesValues := fmt.Sprintf(`0+0x%d`, (t.Status.CompletionTime.Sub(t.Status.StartTime.Time)/(5*time.Second))+2)
	return p.document.PipeE(
		yaml.Lookup("tests"),
		yaml.Append(&yaml.Node{
			Kind: yaml.MappingNode,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "name"},
				{Kind: yaml.ScalarNode, Value: t.Name},

				{Kind: yaml.ScalarNode, Value: "interval", HeadComment: "\n"},
				{Kind: yaml.ScalarNode, Value: "5s", LineComment: "Scrape interval"},

				{Kind: yaml.ScalarNode, Value: "input_series"},
				{
					Kind: yaml.SequenceNode,
					Content: []*yaml.Node{
						{
							Kind: yaml.MappingNode,
							Content: []*yaml.Node{
								{Kind: yaml.ScalarNode, Value: "series"},
								{Kind: yaml.ScalarNode, Value: seriesName, Style: yaml.SingleQuotedStyle},

								{Kind: yaml.ScalarNode, Value: "values"},
								{Kind: yaml.ScalarNode, Value: seriesValues, Style: yaml.SingleQuotedStyle},
							},
						},
					},
				},

				{Kind: yaml.ScalarNode, Value: "promql_expr_test", HeadComment: "\n"},
				{Kind: yaml.SequenceNode},
			},
		}),
	)
}

func (p *promTest) addPromqlExprTest(t *optimizev1beta2.Trial, m *optimizev1beta2.Metric, q string) error {
	return p.document.PipeE(
		yaml.Lookup("tests", "[name="+t.Name+"]", "promql_expr_test"),
		yaml.Append(&yaml.Node{
			Kind:        yaml.MappingNode,
			HeadComment: "\n" + m.Name,
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "expr"},
				{Kind: yaml.ScalarNode, Value: strings.TrimSpace(q)},

				{Kind: yaml.ScalarNode, Value: "eval_time"},
				{Kind: yaml.ScalarNode, Value: t.Status.CompletionTime.Sub(t.Status.StartTime.Time).String()},

				{Kind: yaml.ScalarNode, Value: "exp_samples"},
				{
					Kind: yaml.SequenceNode,
					Content: []*yaml.Node{
						{
							Kind: yaml.MappingNode,
							Content: []*yaml.Node{
								{Kind: yaml.ScalarNode, Value: "labels"},
								{Kind: yaml.ScalarNode, Value: "", Style: yaml.SingleQuotedStyle},
								{Kind: yaml.ScalarNode, Value: "value"},
								{Kind: yaml.ScalarNode, Value: "0", Tag: yaml.NodeTagInt},
							},
						},
					},
				},
			},
		}),
	)
}

func (p *promTest) addQueryComment(m *optimizev1beta2.Metric, q string) {
	switch m.Type {
	case optimizev1beta2.MetricJSONPath:
		p.addHeadComment("%s: url=%s jsonpath=%s", m.Name, m.URL, q)
	default:
		p.addHeadComment("%s: %s", m.Name, q)
	}
}

func (p *promTest) addErrorComment(m *optimizev1beta2.Metric, err error) {
	p.addHeadComment("%s: Error: %s", m.Name, err.Error())
}
