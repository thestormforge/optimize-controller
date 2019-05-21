package trial

import (
	"bytes"
	"fmt"
	okeanosv1alpha1 "github.com/gramLabs/okeanos/pkg/apis/okeanos/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"text/template"
)

type patchContext struct {
	Values map[string]interface{}
}

type metricContext struct {
}

func executePatchTemplate(p *okeanosv1alpha1.PatchTemplate, trial *okeanosv1alpha1.Trial) (types.PatchType, []byte, error) {
	// Determine the patch type
	var patchType types.PatchType
	switch p.Type {
	case "json":
		patchType = types.JSONPatchType
	case "merge":
		patchType = types.MergePatchType
	case "strategic":
		patchType = types.StrategicMergePatchType
	default:
		return "", nil, fmt.Errorf("unknown patch type: %s", p.Type)
	}

	// Create the functions and data for template evaluation
	funcMap := template.FuncMap{}
	data := patchContext{
		Values: trial.Spec.Assignments,
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

func executeMetricQuery(m *okeanosv1alpha1.MetricQuery, trial *okeanosv1alpha1.Trial) (string, error) {
	// Create the functions and data for template evaluation
	funcMap := template.FuncMap{
		"duration": templateDuration,
	}
	data := metricContext{}

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

func templateDuration(start, completion metav1.Time) float64 {
	return completion.Sub(start.Time).Seconds()
}
