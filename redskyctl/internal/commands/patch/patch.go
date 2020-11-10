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

package patch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	redsky "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/internal/experiment"
	"github.com/redskyops/redskyops-controller/internal/patch"
	"github.com/redskyops/redskyops-controller/internal/server"
	"github.com/redskyops/redskyops-controller/internal/template"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/kustomize"
	"github.com/redskyops/redskyops-go/pkg/config"
	experimentsapi "github.com/redskyops/redskyops-go/pkg/redskyapi/experiments/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/resid"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// Options are the configuration options for creating a patched experiment
type Options struct {
	// Config is the Red Sky Configuration used to generate the controller installation
	Config *config.RedSkyConfig
	// ExperimentsAPI is used to interact with the Red Sky Experiments API
	ExperimentsAPI experimentsapi.API
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	inputFiles  []string
	trialNumber int
}

// NewCommand creates a command for performing a patch
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patch",
		Short: "Create a patched manifest using trial parameters",
		Long:  "Create a patched manifest using the parameters from the specified trial",

		PreRunE: func(cmd *cobra.Command, args []string) error {
			commander.SetStreams(&o.IOStreams, cmd)

			var err error
			if o.ExperimentsAPI == nil {
				err = commander.SetExperimentsAPI(&o.ExperimentsAPI, o.Config, cmd)
			}

			return err
		},
		RunE: commander.WithContextE(o.patch),
	}

	cmd.Flags().StringSliceVar(&o.inputFiles, "file", []string{""}, "experiment and related manifests to patch, - for stdin")
	cmd.Flags().IntVar(&o.trialNumber, "trialnumber", -1, "trial number")

	return cmd
}

func (o *Options) patch(ctx context.Context) error {
	if o.trialNumber == -1 {
		return fmt.Errorf("a trial number must be specified")
	}

	experimentBytes, manifestsBytes, err := o.readInputs()
	if err != nil {
		return err
	}

	exp := &redsky.Experiment{}
	rr := commander.NewResourceReader()
	if err := rr.ReadInto(ioutil.NopCloser(bytes.NewReader(experimentBytes)), exp); err != nil {
		return err
	}

	// Populate list of assets to write to kustomize
	assets := map[string]*kustomize.Asset{
		"resources.yaml": kustomize.NewAssetFromBytes(manifestsBytes),
	}

	// look up trial from api
	trialItem, err := o.GetTrialByID(ctx, exp.Name, o.trialNumber)
	if err != nil {
		return err
	}

	trial := &redsky.Trial{}
	experiment.PopulateTrialFromTemplate(exp, trial)
	server.ToClusterTrial(trial, &trialItem.TrialAssignments)

	// render patches
	var patches map[string]types.Patch
	patches, err = createKustomizePatches(exp.Spec.Patches, trial)
	if err != nil {
		return err
	}

	yamls, err := kustomize.Yamls(
		kustomize.WithResources(assets),
		kustomize.WithPatches(patches),
	)
	if err != nil {
		return err
	}

	fmt.Fprintln(o.Out, string(yamls))

	return nil
}

func (o *Options) GetTrialByID(ctx context.Context, experimentName string, trialNumber int) (*experimentsapi.TrialItem, error) {
	query := &experimentsapi.TrialListQuery{
		Status: []experimentsapi.TrialStatus{experimentsapi.TrialCompleted},
	}

	trialList, err := o.getTrials(ctx, experimentName, query)
	if err != nil {
		return nil, err
	}

	// Isolate the given trial we want by number
	var wantedTrial *experimentsapi.TrialItem
	for _, trial := range trialList.Trials {
		if trial.Number == int64(trialNumber) {
			wantedTrial = &trial
			break
		}
	}

	if wantedTrial == nil {
		return nil, fmt.Errorf("trial not found")
	}

	return wantedTrial, nil
}

// getTrials gets all trials from the redsky api for a given experiment.
func (o *Options) getTrials(ctx context.Context, experimentName string, query *experimentsapi.TrialListQuery) (trialList experimentsapi.TrialList, err error) {
	if o.ExperimentsAPI == nil {
		return trialList, fmt.Errorf("unable to connect to api server")
	}

	experiment, err := o.ExperimentsAPI.GetExperimentByName(ctx, experimentsapi.NewExperimentName(experimentName))
	if err != nil {
		fmt.Println("failed here", err)
		return trialList, err
	}

	if experiment.TrialsURL == "" {
		return trialList, fmt.Errorf("unable to identify trial")
	}

	return o.ExperimentsAPI.GetAllTrials(ctx, experiment.TrialsURL, query)
}

// readInputs handles all of the loading of files and/or stdin. It utilizes kio.pipelines
// so we can better handle reading from stdin and getting at the specific data we need.
func (o *Options) readInputs() (experiment []byte, manifests []byte, err error) {
	kioInputs := []kio.Reader{}

	for _, filename := range o.inputFiles {
		r, err := o.IOStreams.OpenFile(filename)
		if err != nil {
			return nil, nil, err
		}
		defer r.Close()

		kioInputs = append(kioInputs, &kio.ByteReader{Reader: bufio.NewReader(r)})
	}

	var (
		inputsBuf     bytes.Buffer
		manifestsBuf  bytes.Buffer
		experimentBuf bytes.Buffer
	)

	// Read in all inputs
	// // We read in everything at first so we only read o.In once
	allInput := kio.Pipeline{
		Inputs:  kioInputs,
		Outputs: []kio.Writer{kio.ByteWriter{Writer: &inputsBuf}},
	}
	if err := allInput.Execute(); err != nil {
		return experiment, manifests, err
	}

	// Render manifests
	manifestsInput := kio.Pipeline{
		Inputs:  []kio.Reader{&kio.ByteReader{Reader: bytes.NewReader(inputsBuf.Bytes())}},
		Filters: []kio.Filter{kio.FilterFunc(filterRemoveExperiment)},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: &manifestsBuf}},
	}
	if err := manifestsInput.Execute(); err != nil {
		return experiment, manifests, err
	}

	// Render Experiment
	experimentInput := kio.Pipeline{
		Inputs:  []kio.Reader{&kio.ByteReader{Reader: bytes.NewReader(inputsBuf.Bytes())}},
		Filters: []kio.Filter{kio.FilterFunc(filterSaveExperiment)},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: &experimentBuf}},
	}
	if err := experimentInput.Execute(); err != nil {
		return experiment, manifests, err
	}

	return experimentBuf.Bytes(), manifestsBuf.Bytes(), nil
}

// filterRemoveExperiment is used to strip experiments from the inputs.
func filterRemoveExperiment(input []*yaml.RNode) ([]*yaml.RNode, error) {
	var output kio.ResourceNodeSlice
	for i := range input {
		m, err := input[i].GetMeta()
		if err != nil {
			return nil, err
		}
		if m.Kind == "Experiment" {
			continue
		}
		output = append(output, input[i])
	}
	return output, nil
}

// filterSaveExperiment is used to strip everything but experiments from the inputs.
func filterSaveExperiment(input []*yaml.RNode) ([]*yaml.RNode, error) {
	var output kio.ResourceNodeSlice
	for i := range input {
		m, err := input[i].GetMeta()
		if err != nil {
			return nil, err
		}
		if m.Kind != "Experiment" {
			continue
		}
		output = append(output, input[i])
	}
	return output, nil
}

// createKustomizePatches translates a patchTemplate into a kustomize (json) patch
func createKustomizePatches(patchSpec []redsky.PatchTemplate, trial *redsky.Trial) (map[string]types.Patch, error) {
	te := template.New()
	patches := map[string]types.Patch{}

	for idx, expPatch := range patchSpec {
		ref, data, err := patch.RenderTemplate(te, trial, &expPatch)
		if err != nil {
			return nil, err
		}

		// Surely there's got to be a better way
		// // Transition patch from json to map[string]interface
		m := make(map[string]interface{})
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}

		u := &unstructured.Unstructured{}
		// // Set patch data first ( otherwise it overwrites everything else )
		u.SetUnstructuredContent(m)
		// // Define object/type meta
		u.SetName(ref.Name)
		u.SetNamespace(ref.Namespace)
		u.SetGroupVersionKind(ref.GroupVersionKind())
		// // Profit
		b, err := u.MarshalJSON()
		if err != nil {
			return nil, err
		}

		patches[fmt.Sprintf("%s-%d", "patch", idx)] = types.Patch{
			Patch: string(b),
			Target: &types.Selector{
				Gvk: resid.Gvk{
					Group:   ref.GroupVersionKind().Group,
					Version: ref.GroupVersionKind().Version,
					Kind:    ref.GroupVersionKind().Kind,
				},
				Name:      ref.Name,
				Namespace: ref.Namespace,
			},
		}
	}

	return patches, nil
}
