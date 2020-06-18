package export_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	redsky "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/export"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExport(t *testing.T) {
	samplePatch := `spec:
  template:
    spec:
      containers:
        - name: postgres
          resources:
          limits:
            cpu: "{{ .Values.cpu }}m"
            memory: "{{ .Values.memory }}Mi"
          requests:
            cpu: "{{ .Values.cpu }}m"
            memory: "{{ .Values.memory }}Mi"`

	sampleExperiment := &redsky.Experiment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sampleExperiment",
			Namespace: "default",
		},
		Spec: redsky.ExperimentSpec{
			Parameters: []redsky.Parameter{},
			Metrics:    []redsky.Metric{},
			Patches: []redsky.PatchTemplate{
				{
					TargetRef: &corev1.ObjectReference{
						Kind:       "Deployment",
						APIVersion: "apps/v1",
						Name:       "postgres",
					},
					Patch: samplePatch,
				},
			},
			TrialTemplate: redsky.TrialTemplateSpec{},
		},
		Status: redsky.ExperimentStatus{},
	}

	tmpfile, err := ioutil.TempFile("", "experiment")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	b, err := json.Marshal(sampleExperiment)
	require.NoError(t, err)
	_, err = tmpfile.Write(b)
	require.NoError(t, err)

	testCases := []struct {
		desc string
		args []string
	}{
		{
			desc: "default",
			args: []string{
				"als-368",
				"--filename",
				tmpfile.Name(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {

			var b bytes.Buffer
			cmd := export.NewCommand()
			cmd.SetOut(&b)

			if len(tc.args) > 0 {
				cmd.SetArgs(tc.args)
			}

			err := cmd.Execute()
			assert.NoError(t, err)
			log.Println(b.String())
		})
	}
}
