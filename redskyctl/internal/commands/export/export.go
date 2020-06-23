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

package export

import (
	"context"
	"fmt"
	"path/filepath"

	redsky "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/internal/patch"
	"github.com/redskyops/redskyops-controller/internal/server"
	expapi "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/experiments"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/config"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/util"
	"github.com/spf13/cobra"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
)

func NewCommand(cfg config.Config) *cobra.Command {
	ec := &ExportCommand{
		Config: cfg,
	}

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export trial parameters",
		Long:  "Export trial parameters to a Kubernetes object",
		RunE:  ec.Run,
	}

	// TODO mark flag as required
	cmd.Flags().StringVarP(&ec.filename, "filename", "f", "", "path to experiment file")

	return cmd
}

type ExportCommand struct {
	Config    config.Config
	RedSkyAPI expapi.API
	filename  string

	experiment *redsky.Experiment
	trial      *redsky.Trial
}

func (e *ExportCommand) Run(cmd *cobra.Command, args []string) (err error) {
	// Set up RSO client
	api, err := commander.NewExperimentsAPI(cmd.Context(), e.Config)
	if err != nil {
		return err
	}
	e.RedSkyAPI = api

	// Read the experiment
	if err = e.ReadExperimentFile(); err != nil {
		return err
	}

	// Discover all trials for a given experiment
	trialNames, err := experiments.ParseNames(append([]string{"trials"}, args...))
	if err != nil {
		return err
	}

	if len(trialNames) != 1 {
		return fmt.Errorf("only a single trial name is supported")
	}

	// Get parameters for given trial
	trial, err := e.GetTrial(cmd.Context(), trialNames[0])
	if err != nil {
		return err
	}

	e.trial = trial

	// Render all the necessary patches defined in the experiment
	// with the parameters from the trial
	patches, err := e.Patches(cmd.Context())
	if err != nil {
		return err
	}

	resources, err := patchResource(patches)
	if err != nil {
		return err
	}

	fmt.Println(string(resources))

	return nil
}

func (e *ExportCommand) ReadExperimentFile() (err error) {
	var experimentFile *redsky.Experiment
	experimentFile, err = util.ReadExperiment(e.filename, nil)
	if err != nil {
		return err
	}

	e.experiment = experimentFile

	return nil
}

func (e *ExportCommand) GetTrial(ctx context.Context, trialID experiments.Identifier) (trial *redsky.Trial, err error) {
	if e.RedSkyAPI == nil {
		return nil, fmt.Errorf("unable to connect to api server")
	}

	experiment, err := e.RedSkyAPI.GetExperimentByName(ctx, trialID.ExperimentName())
	if err != nil {
		return trial, err
	}

	query := &expapi.TrialListQuery{
		Status: []expapi.TrialStatus{expapi.TrialCompleted},
	}

	if experiment.TrialsURL == "" {
		return trial, fmt.Errorf("unable to identify trial")
	}

	trialList, err := e.RedSkyAPI.GetAllTrials(ctx, experiment.TrialsURL, query)
	if err != nil {
		return trial, err
	}

	// Isolate the given trial we want by number
	var wantedTrial *expapi.TrialItem
	for _, trial := range trialList.Trials {
		if trial.Number == trialID.Number {
			wantedTrial = &trial
			break
		}
	}

	if wantedTrial == nil {
		return trial, fmt.Errorf("trial not found")
	}

	// Convert api trial to kube trial
	trial = redskyTrial(wantedTrial, trialID.ExperimentName().Name(), e.experiment.ObjectMeta.Namespace)

	return trial, nil
}

func redskyTrial(apiTrial *expapi.TrialItem, expName string, expNamespace string) (trial *redsky.Trial) {
	trial = &redsky.Trial{}
	server.ToClusterTrial(trial, &apiTrial.TrialAssignments)
	trial.ObjectMeta.Labels = map[string]string{
		redsky.LabelExperiment: expName,
	}

	// Is this actually needed?
	trial.ObjectMeta.Namespace = expNamespace

	return trial
}

type patchNTarget struct {
	// This doesnt seem like the best idea, but we'll assume it is
	Target     []byte
	TargetName string
	Patch      types.Patch
	PatchName  string
}

func (e *ExportCommand) Patches(ctx context.Context) ([]*patchNTarget, error) {
	// Generate patch operations
	patcher := patch.NewPatcher()
	for _, patch := range e.experiment.Spec.Patches {
		po, err := patcher.CreatePatchOperation(e.trial, &patch)
		if err != nil {
			return nil, err
		}

		if po == nil {
			// TODO figure out how to find name
			// Maybe just toss out experiment name
			return nil, fmt.Errorf("failed to create a patch operation for %s", e.trial.ObjectMeta.GenerateName)
		}

		e.trial.Status.PatchOperations = append(e.trial.Status.PatchOperations, *po)
	}

	// Apply patch operations
	patches := []*patchNTarget{}
	for idx, patchOp := range e.trial.Status.PatchOperations {
		kubectlGet, err := e.Config.Kubectl(ctx,
			"-n",
			patchOp.TargetRef.Namespace,
			"get",
			patchOp.TargetRef.Kind,
			patchOp.TargetRef.Name,
			"-o",
			"json")
		if err != nil {
			return nil, err
		}

		output, err := kubectlGet.CombinedOutput()
		if err != nil {
			// TODO: Should we wrap the error with the output
			// return nil, fmt.Errorf("kubectl failed with %v: %w", string(output), err)
			return nil, err
		}

		patchBytes, err := createPatch(&patchOp, output)
		if err != nil {
			return nil, err
		}

		p := types.Patch{
			Patch: string(patchBytes),
			Target: &types.Selector{
				Name:      patchOp.TargetRef.Name,
				Namespace: patchOp.TargetRef.Namespace,
			},
		}

		pnt := &patchNTarget{
			Target:     output,
			TargetName: fmt.Sprintf("%s-%s-%s", patchOp.TargetRef.Namespace, patchOp.TargetRef.Kind, patchOp.TargetRef.Name),
			Patch:      p,
			PatchName:  fmt.Sprintf("%s-%d", "patch", idx),
		}

		patches = append(patches, pnt)
	}

	return patches, nil
}

func patchResource(pnt []*patchNTarget) ([]byte, error) {
	k := &types.Kustomization{}

	// Set up a kustomize target
	// TODO see how we can integrate this with kustomize pkg
	fs := filesys.MakeFsInMemory()
	base := "/app/base"

	var err error

	for _, p := range pnt {
		if err = fs.WriteFile(filepath.Join(base, p.PatchName), []byte(p.Patch.Patch)); err != nil {
			return nil, err
		}

		k.Patches = append(k.Patches, p.Patch)

		// Only write target out once
		if exists := fs.Exists(filepath.Join(base, p.TargetName)); !exists {
			if err = fs.WriteFile(filepath.Join(base, p.TargetName), p.Target); err != nil {
				return nil, err
			}
			k.Resources = append(k.Resources, p.TargetName)
		}
	}

	kYaml, err := yaml.Marshal(k)
	if err != nil {
		return nil, err
	}

	if err = fs.WriteFile(filepath.Join(base, konfig.DefaultKustomizationFileName()), kYaml); err != nil {
		return nil, err
	}

	kustomizer := krusty.MakeKustomizer(fs, krusty.MakeDefaultOptions())
	res, err := kustomizer.Run(base)
	if err != nil {
		return nil, err
	}

	return res.AsYaml()
}
