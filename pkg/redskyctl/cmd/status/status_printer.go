package status

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	"sigs.k8s.io/yaml"
)

var _ TrialStatusPrinter = &tablePrinter{}
var _ TrialStatusPrinter = &jsonYamlPrinter{}
var _ TrialStatusPrinter = &namePrinter{}

type tablePrinter struct {
}

func (p *tablePrinter) PrintTrialListStatus(trials *v1alpha1.TrialList, w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)

	// Collect distinct namespaces
	namespaces := make(map[string]bool)
	for i := range trials.Items {
		namespaces[trials.Items[i].Namespace] = true
	}

	// Compute and format the header
	var header []string
	if len(namespaces) > 1 {
		header = append(header, "NAMESPACE")
	}
	header = append(header, "NAME", "STATUS")

	if _, err := fmt.Fprintln(tw, strings.Join(header, "\t")); err != nil {
		return err
	}

	// Compute and format each row
	for i := range trials.Items {
		var row []string
		if len(namespaces) > 1 {
			row = append(row, trials.Items[i].Namespace)
		}
		row = append(row, trials.Items[i].Name)
		row = append(row, summarize(&trials.Items[i].Status))

		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}

	return tw.Flush()
}

type jsonYamlPrinter struct {
	yaml bool
}

func (p *jsonYamlPrinter) PrintTrialListStatus(trials *v1alpha1.TrialList, w io.Writer) error {
	// Format this as a map of trial names to trial status instances
	o := make(map[string]*v1alpha1.TrialStatus, len(trials.Items))
	for i := range trials.Items {
		o[trials.Items[i].Name] = &trials.Items[i].Status
	}

	if p.yaml {
		output, err := yaml.Marshal(o)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, string(output))
		return err
	}

	data, err := json.MarshalIndent(o, "", "    ")
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

type namePrinter struct {
}

func (p *namePrinter) PrintTrialListStatus(trials *v1alpha1.TrialList, w io.Writer) error {
	for i := range trials.Items {
		if _, err := fmt.Fprintf(w, "%s\n", trials.Items[i].Name); err != nil {
			return err
		}
	}
	return nil
}
