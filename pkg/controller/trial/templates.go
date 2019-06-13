package trial

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	okeanosv1alpha1 "github.com/gramLabs/okeanos/pkg/apis/okeanos/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

type patchContext struct {
	Values map[string]string
}

type metricContext struct {
	// The time at which the trial run started (possibly adjusted)
	StartTime time.Time
	// The time at which the trial run completed
	CompletionTime time.Time
	// The duration of the trial run expressed as a Prometheus range value
	Range string
}

func executePatchTemplate(p *okeanosv1alpha1.PatchTemplate, trial *okeanosv1alpha1.Trial) (types.PatchType, []byte, error) {
	// Determine the patch type
	var patchType types.PatchType
	switch p.Type {
	case "json":
		patchType = types.JSONPatchType
	case "merge":
		patchType = types.MergePatchType
	case "strategic", "":
		patchType = types.StrategicMergePatchType
	default:
		return "", nil, fmt.Errorf("unknown patch type: %s", p.Type)
	}

	// Create the functions map
	funcMap := template.FuncMap{}

	// Create the data context
	data := patchContext{}
	data.Values = make(map[string]string, len(trial.Spec.Assignments))
	for _, a := range trial.Spec.Assignments {
		data.Values[a.Name] = a.Value
	}

	// Evaluate the template into a patch
	tmpl, err := template.New("patch").Funcs(funcMap).Parse(p.Patch)
	if err != nil {
		return "", nil, err
	}
	buf := new(bytes.Buffer)
	if err = tmpl.Execute(buf, data); err != nil {
		return "", nil, err
	}
	return patchType, buf.Bytes(), nil
}

func executeMetricQueryTemplate(m *okeanosv1alpha1.Metric, trial *okeanosv1alpha1.Trial) (string, error) {
	// Create the functions map
	funcMap := template.FuncMap{
		"duration": templateDuration,
	}

	// Create the data context
	data := metricContext{}
	if trial.Status.StartTime != nil {
		data.StartTime = trial.Status.StartTime.Time
	}
	if trial.Status.CompletionTime != nil {
		data.CompletionTime = trial.Status.CompletionTime.Time
	}
	data.Range = fmt.Sprintf("%.0fs", templateDuration(data.StartTime, data.CompletionTime))

	// Evaluate the template into a query
	tmpl, err := template.New("query").Funcs(funcMap).Parse(m.Query)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	if err = tmpl.Execute(buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func templateDuration(start, completion time.Time) float64 {
	if start.Before(completion) {
		return completion.Sub(start).Seconds()
	}
	return 0
}
