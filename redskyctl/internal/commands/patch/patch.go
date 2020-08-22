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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"

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
	"sigs.k8s.io/kustomize/api/types"
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

// NewCommand creates a command for performing an initialization
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patch",
		Short: "patchy patch",
		Long:  "patchy patchy poo poo",

		//PreRun: commander.StreamsPreRun(&o.IOStreams),
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

	cmd.Flags().StringSliceVar(&o.inputFiles, "file", []string{""}, "experiment filename")
	cmd.Flags().IntVar(&o.trialNumber, "trialnumber", 0, "trial number")

	return cmd
}

func (o *Options) patch(ctx context.Context) error {
	rr := commander.NewResourceReader()
	exp := &redsky.Experiment{}

	// Read through all files until we get an appropriate experiment file.
	// The first one read will win.
	for idx, filename := range o.inputFiles {
		r, err := o.IOStreams.OpenFile(filename)
		if err != nil {
			return err
		}
		defer r.Close()
		if err := rr.ReadInto(r, exp); err == nil {
			o.inputFiles = append(o.inputFiles[:idx], o.inputFiles[idx+1:]...)
			break
		}
	}

	if exp == nil {
		return fmt.Errorf("unable to open experiment")
	}

	// Populate list of assets to write to kustomize
	assets := map[string]*kustomize.Asset{}
	for _, filename := range o.inputFiles {
		var (
			data  []byte
			input io.ReadCloser
			err   error
		)

		if input, err = o.IOStreams.OpenFile(filename); err != nil {
			return err
		}

		if data, err = ioutil.ReadAll(input); err != nil {
			return err
		}

		if len(data) == 0 {
			fmt.Println("warning, read empty file", filename)
			continue
		}

		asset := kustomize.NewAssetFromBytes(data)
		assets[filepath.Base(filename)] = asset
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
	te := template.New()
	patches := map[string]types.Patch{}
	for idx, expPatch := range exp.Spec.Patches {
		ref, data, err := patch.RenderTemplate(te, trial, &expPatch)
		if err != nil {
			return err
		}

		// Surely there's got to be a better way
		// // Transition patch from json to map[string]interface
		m := make(map[string]interface{})
		if err := json.Unmarshal(data, &m); err != nil {
			return err
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
			return err
		}

		patches[fmt.Sprintf("%s-%d", "patch", idx)] = types.Patch{
			Patch: string(b),
			Target: &types.Selector{
				Name:      ref.Name,
				Namespace: ref.Namespace,
			},
		}
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

	//o.trial = redskyTrial(wantedTrial, trialID.ExperimentName().Name(), e.experimentctl.ObjectMeta.Namespace)
	// Convert api trial to kube trial
	return wantedTrial, nil
}

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
