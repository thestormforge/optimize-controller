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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thestormforge/konjure/pkg/filters"
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/kustomize"
	"github.com/thestormforge/optimize-controller/v2/internal/application"
	"github.com/thestormforge/optimize-controller/v2/internal/experiment"
	"github.com/thestormforge/optimize-controller/v2/internal/patch"
	"github.com/thestormforge/optimize-controller/v2/internal/scan"
	"github.com/thestormforge/optimize-controller/v2/internal/server"
	"github.com/thestormforge/optimize-controller/v2/internal/sfio"
	"github.com/thestormforge/optimize-controller/v2/internal/template"
	"github.com/thestormforge/optimize-go/pkg/api"
	applicationsv2 "github.com/thestormforge/optimize-go/pkg/api/applications/v2"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"github.com/thestormforge/optimize-go/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/resid"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// Options are the configuration options for creating a patched experiment
type Options struct {
	// Config is the Optimize Configuration used to generate the controller installation
	Config *config.OptimizeConfig
	// ExperimentsAPI is used to interact with the Optimize Experiments API
	ExperimentsAPI experimentsv1alpha1.API
	// ApplicationsAPI is used to interact with the Optimize Applications API
	ApplicationsAPI applicationsv2.API
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	inputFiles         []string
	trialName          string
	recommendationName string
	patchOnly          bool
	patchedTarget      bool

	// This is used for testing
	Fs          filesys.FileSystem
	inputData   []byte
	experiment  *optimizev1beta2.Experiment
	application *optimizeappsv1alpha1.Application
	resources   map[string]struct{}
}

// trialDetails contains information about a trial collected from the Experiments API.
type trialDetails struct {
	Assignments *experimentsv1alpha1.TrialAssignments
	Experiment  string
	Application string
	Scenario    string
	Objective   string
}

type recommendationDetails struct {
	Recommendation *applicationsv2.Recommendation
}

// NewCommand creates a command for performing an export
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export TRIAL_NAME|APP_NAME|REC_NAME",
		Short: "Export or apply a patch",
		Long:  "Export trial parameters or a recommendation as a patch",

		PreRunE: func(cmd *cobra.Command, args []string) error {
			commander.SetStreams(&o.IOStreams, cmd)

			var err error
			if o.ExperimentsAPI == nil {
				err = commander.SetExperimentsAPI(&o.ExperimentsAPI, o.Config, cmd)
			}
			if o.ApplicationsAPI == nil {
				err = commander.SetApplicationsAPI(&o.ApplicationsAPI, o.Config, cmd)
			}

			if len(args) != 1 || args[0] == "" {
				return fmt.Errorf("a name (trial, application, or recommendation) must be specified")
			}

			// Inspect the argument and try to figure if it is a trial or recommendation
			_, rn := applicationsv2.SplitRecommendationName(args[0])
			if _, err := strconv.ParseInt(rn, 10, 64); err != nil && rn != "" {
				o.recommendationName = args[0]
			} else if _, num := experimentsv1alpha1.SplitTrialName(args[0]); num >= 0 {
				o.trialName = args[0]
			} else {
				o.recommendationName = args[0]
			}

			return err
		},
		RunE: commander.WithContextE(o.runner),
	}

	cmd.Flags().StringSliceVarP(&o.inputFiles, "filename", "f", nil, "experiment and related manifest `files` to export, - for stdin")
	cmd.Flags().BoolVarP(&o.patchOnly, "patch", "p", false, "export only the patch")
	cmd.Flags().BoolVarP(&o.patchedTarget, "patched-target", "t", false, "export only the patched resource")

	_ = cmd.MarkFlagFilename("filename", "yml", "yaml")

	return cmd
}

func (o *Options) readInput() error {
	// Do an in memory filesystem so we can properly handle stdin
	if o.Fs == nil {
		o.Fs = filesys.MakeFsInMemory()
	}

	if o.resources == nil {
		o.resources = make(map[string]struct{})
	}

	kioInputs := []kio.Reader{}

	for _, filename := range o.inputFiles {
		r, err := o.IOStreams.OpenFile(filename)
		if err != nil {
			return err
		}
		defer r.Close()

		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		if filename == "-" {
			filename = "stdin.yaml"
		}

		if err := o.Fs.WriteFile(filepath.Base(filename), data); err != nil {
			return err
		}

		path, err := filepath.Abs(filename)
		if err != nil {
			return err
		}

		kioInputs = append(kioInputs, &kio.ByteReader{
			Reader: bytes.NewReader(data),
			SetAnnotations: map[string]string{
				kioutil.PathAnnotation: path,
			},
		})

		// Track all of the input files so we can use them as kustomize resources later on
		o.resources[filepath.Base(filename)] = struct{}{}
	}

	var inputsBuf bytes.Buffer

	// Aggregate all inputs
	allInput := kio.Pipeline{
		Inputs:  kioInputs,
		Outputs: []kio.Writer{kio.ByteWriter{Writer: &inputsBuf}},
	}
	if err := allInput.Execute(); err != nil {
		return err
	}

	o.inputData = inputsBuf.Bytes()

	return nil
}

func (o *Options) extractApplication(trial *trialDetails) error {
	var appBuf bytes.Buffer

	// Render Experiment
	appInput := kio.Pipeline{
		Inputs:  []kio.Reader{&kio.ByteReader{Reader: bytes.NewReader(o.inputData)}},
		Filters: []kio.Filter{&filters.ResourceMetaFilter{Group: optimizeappsv1alpha1.GroupVersion.Group, Kind: "Application", Name: trial.Application}},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: &appBuf}},
	}
	if err := appInput.Execute(); err != nil {
		return err
	}

	// We don't want to bail if we cant find an application since we'll handle this later
	if appBuf.Len() == 0 {
		return nil
	}

	o.application = &optimizeappsv1alpha1.Application{}

	return commander.NewResourceReader().ReadInto(io.NopCloser(&appBuf), o.application)
}

func (o *Options) extractExperiment(trial *trialDetails) error {
	var experimentBuf bytes.Buffer

	// Render Experiment
	experimentInput := kio.Pipeline{
		Inputs:  []kio.Reader{&kio.ByteReader{Reader: bytes.NewReader(o.inputData)}},
		Filters: []kio.Filter{&filters.ResourceMetaFilter{Group: optimizev1beta2.GroupVersion.Group, Kind: "Experiment", Name: trial.Experiment}},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: &experimentBuf}},
	}
	if err := experimentInput.Execute(); err != nil {
		return err
	}

	// We don't want to bail if we cant find an experiment since we'll handle this later
	if experimentBuf.Len() == 0 {
		return nil
	}

	o.experiment = &optimizev1beta2.Experiment{}

	return commander.NewResourceReader().ReadInto(io.NopCloser(&experimentBuf), o.experiment)
}

// filter returns a filter function to exctract a specified `kind` from the input.
func filterPatch(patches []types.Patch) kio.FilterFunc {
	return func(input []*yaml.RNode) ([]*yaml.RNode, error) {
		var output kio.ResourceNodeSlice

		for i := range input {
			m, err := input[i].GetMeta()
			if err != nil {
				return nil, err
			}
			for _, patch := range patches {
				// Skip comparison if patch.Target.X is ""
				if patch.Target.Kind != "" && patch.Target.Kind != m.Kind {
					continue
				}

				gv := strings.Split(m.APIVersion, "/")
				if len(gv) != 2 {
					continue
				}

				if patch.Target.Group != "" && patch.Target.Group != gv[0] {
					continue
				}

				if patch.Target.Version != "" && patch.Target.Version != gv[1] {
					continue
				}

				if patch.Target.Name != "" && patch.Target.Name != m.Name {
					continue
				}

				output = append(output, input[i])
			}
		}
		return output, nil
	}
}

func (o *Options) runner(ctx context.Context) error {
	if err := o.readInput(); err != nil {
		return err
	}

	// look up trial from api
	trialDetails, err := o.getTrialDetails(ctx)
	if err != nil {
		return err
	}

	var recDetails *recommendationDetails
	if trialDetails != nil {
		// See if we have been given an experiment
		if err := o.extractExperiment(trialDetails); err != nil {
			return fmt.Errorf("got an error when looking for experiment: %w", err)
		}

		// Still no experiment, we may need to generate it from an application
		if o.experiment == nil {
			if err := o.extractApplication(trialDetails); err != nil {
				return fmt.Errorf("got an error when looking for application: %w", err)
			}

			if o.application == nil {
				return fmt.Errorf("unable to find an application %q", trialDetails.Application)
			}

			if err := o.generateExperiment(trialDetails); err != nil {
				return err
			}
		}
	} else {
		recDetails, err = o.getRecommendationDetails(ctx)
		if err != nil {
			return err
		}
	}

	var patches []types.Patch
	if trialDetails != nil {
		if o.experiment == nil {
			return fmt.Errorf("unable to find an experiment %q", trialDetails.Experiment)
		}

		trial := &optimizev1beta2.Trial{}
		experiment.PopulateTrialFromTemplate(o.experiment, trial)
		server.ToClusterTrial(trial, trialDetails.Assignments)

		// render patches
		if pp, err := createTrialKustomizePatches(o.experiment.Spec.Patches, trial); err != nil {
			return err
		} else {
			patches = append(patches, pp...)
		}
	}
	if recDetails != nil {
		// render patches
		mapper := o.mapper(ctx)
		if pp, err := createRecommendationKustomizePatches(mapper, recDetails.Recommendation.Parameters); err != nil {
			return err
		} else {
			patches = append(patches, pp...)
		}
	}

	if o.patchOnly {
		for _, p := range patches {
			fmt.Fprintln(o.Out, p.Patch)
		}

		return nil
	}

	resourceNames := make([]string, 0, len(o.resources))
	for name := range o.resources {
		resourceNames = append(resourceNames, name)
	}

	yamls, err := kustomize.Yamls(
		kustomize.WithFS(o.Fs),
		kustomize.WithResourceNames(resourceNames),
		kustomize.WithPatches(patches),
	)
	if err != nil {
		return err
	}

	if !o.patchedTarget {
		fmt.Fprintln(o.Out, string(yamls))
		return nil
	}

	return kio.Pipeline{
		Inputs:  []kio.Reader{&kio.ByteReader{Reader: bytes.NewReader(yamls)}},
		Filters: []kio.Filter{filterPatch(patches)},
		Outputs: []kio.Writer{o.YAMLWriter()},
	}.Execute()
}

func (o *Options) generateExperiment(trial *trialDetails) error {
	list := &corev1.List{}

	opts := scan.FilterOptions{
		DefaultReader: o.In,
	}

	gen := experiment.Generator{
		Application:    *o.application,
		ExperimentName: trial.Experiment,
		Scenario:       trial.Scenario,
		Objective:      trial.Objective,
		FilterOptions:  opts,
	}

	if gen.Scenario == "" && gen.Objective == "" {
		gen.Scenario, gen.Objective = application.GuessScenarioAndObjective(&gen.Application, gen.ExperimentName)
	}

	if err := gen.Execute((*sfio.ObjectList)(list)); err != nil {
		return err
	}

	// Reset/Restrict application resources to only those specified by the application
	// and resources generated by the generator
	o.resources = make(map[string]struct{})

	for idx := range list.Items {
		listBytes, err := list.Items[idx].MarshalJSON()
		if err != nil {
			return err
		}

		assetName := fmt.Sprintf("%s%d%s", "application-assets", idx, ".yaml")
		if err := o.Fs.WriteFile(assetName, listBytes); err != nil {
			return err
		}

		o.resources[assetName] = struct{}{}

		if te, ok := list.Items[idx].Object.(*optimizev1beta2.Experiment); ok {
			o.experiment = &optimizev1beta2.Experiment{}
			te.DeepCopyInto(o.experiment)
		}
	}

	// Load up all application resources
	var buf bytes.Buffer
	err := kio.Pipeline{
		Inputs:  []kio.Reader{o.application.Resources},
		Filters: []kio.Filter{opts.NewFilter(application.WorkingDirectory(o.application))},
		Outputs: []kio.Writer{&kio.ByteWriter{
			Writer: &buf,
		}},
	}.Execute()
	if err != nil {
		return err
	}

	if err := o.Fs.WriteFile("resources.yaml", buf.Bytes()); err != nil {
		return err
	}

	o.resources["resources.yaml"] = struct{}{}

	return nil
}

// getTrialDetails returns information about the requested trial.
func (o *Options) getTrialDetails(ctx context.Context) (*trialDetails, error) {
	if o.trialName == "" {
		return nil, nil
	}
	if o.ExperimentsAPI == nil {
		return nil, fmt.Errorf("unable to connect to api server")
	}

	experimentName, trialNumber := experimentsv1alpha1.SplitTrialName(o.trialName)
	if trialNumber < 0 {
		return nil, fmt.Errorf("invalid trial name %q", o.trialName)
	}

	exp, err := o.ExperimentsAPI.GetExperimentByName(ctx, experimentName)
	if err != nil {
		return nil, err
	}
	if exp.Link(api.RelationTrials) == "" {
		return nil, fmt.Errorf("unable to find trials for experiment")
	}

	// Capture details about the trial provenance
	result := &trialDetails{
		Experiment:  experimentName.String(),
		Application: exp.Labels["application"],
		Scenario:    exp.Labels["scenario"],
		Objective:   exp.Labels["objective"],
	}

	query := experimentsv1alpha1.TrialListQuery{}
	query.SetStatus(experimentsv1alpha1.TrialCompleted)
	trialList, err := o.ExperimentsAPI.GetAllTrials(ctx, exp.Link(api.RelationTrials), query)
	if err != nil {
		return nil, err
	}

	for i := range trialList.Trials {
		if trialList.Trials[i].Number == trialNumber {
			result.Assignments = &trialList.Trials[i].TrialAssignments
			break
		}
	}

	if result.Assignments == nil {
		return nil, fmt.Errorf("trial not found")
	}
	return result, nil
}

// createTrialKustomizePatches translates a patchTemplate into a kustomize (json) patch
func createTrialKustomizePatches(patchSpec []optimizev1beta2.PatchTemplate, trial *optimizev1beta2.Trial) ([]types.Patch, error) {
	te := template.New()
	patches := make([]types.Patch, len(patchSpec))

	for idx, expPatch := range patchSpec {
		ref, data, err := patch.RenderTemplate(te, trial, &expPatch)
		if err != nil {
			return nil, err
		}

		switch expPatch.Type {
		// If json patch, we can consume the patch as is
		case optimizev1beta2.PatchJSON:
		// Otherwise we need to inject the type meta into the patch data
		// because it says so
		// https://github.com/kubernetes-sigs/kustomize/blob/master/examples/inlinePatch.md
		default:
			// Surely there's got to be a better way
			// Trying to go from corev1.ObjectRef -> metav1.PartialObjectMetadata
			// kind of works, but we're unable to really do much with that because
			// the rendered patch we get back from te.RenderPatch is already a json
			// object ( as in it begins/ends with `{ }`. So a simple append(pom, data...)
			// wont work.
			// We could try to go through the whole jump of switch gvk and create explicit
			// objects for each, but that isnt really right or addressing the issue either
			// So instead we'll do this dance with unstructured.

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
			data, err = u.MarshalJSON()
			if err != nil {
				return nil, err
			}
		}

		patches[idx] = types.Patch{
			Patch: string(data),
			Target: &types.Selector{
				KrmId: types.KrmId{
					Gvk: resid.Gvk{
						Group:   ref.GroupVersionKind().Group,
						Version: ref.GroupVersionKind().Version,
						Kind:    ref.GroupVersionKind().Kind,
					},
					Name: ref.Name,
				},
			},
		}
	}

	return patches, nil
}

// getRecommendationDetails fetches the full recommendation from the server.
func (o *Options) getRecommendationDetails(ctx context.Context) (*recommendationDetails, error) {
	if o.recommendationName == "" {
		return nil, nil
	}
	if o.ApplicationsAPI == nil {
		return nil, fmt.Errorf("unable to connect to api server")
	}

	applicationName, recommendationName := applicationsv2.SplitRecommendationName(o.recommendationName)

	app, err := o.ApplicationsAPI.GetApplicationByName(ctx, applicationName)
	if err != nil {
		return nil, err
	}
	if app.Link(api.RelationRecommendations) == "" {
		return nil, fmt.Errorf("unable to find recommendations for application (missing link)")
	}

	result := &recommendationDetails{}

	var recURL string

	if recommendationName != "" {
		u, err := url.Parse(app.Link(api.RelationRecommendations))
		if err != nil {
			return nil, fmt.Errorf("unable to find recommendations for application (bad link): %w", err)
		}
		u.Path = path.Join(u.Path, recommendationName)
		recURL = u.String()
	}

	if recURL == "" {
		recList, err := o.ApplicationsAPI.ListRecommendations(ctx, app.Link(api.RelationRecommendations))
		if err != nil {
			return nil, err
		}
		if len(recList.Recommendations) == 0 {
			return nil, fmt.Errorf("unable to find recommendations for application (none available)")
		}
		recURL = recList.Recommendations[0].Link(api.RelationSelf)

		// TODO This is a dumb bug to have to workaround; the links come with self and up swapped...
		if up := recList.Recommendations[0].Link(api.RelationUp); len(up) > len(recURL) {
			recURL = up
		}
	}

	rec, err := o.ApplicationsAPI.GetRecommendation(ctx, recURL)
	if err != nil {
		return nil, err
	}
	result.Recommendation = &rec

	return result, nil
}

// mapper returns REST mapper using the scheme baked in to the code plus any CRDs
// installed on the server. This code is borrowed from the RBAC generator which
// also needs to map between GVRs and GVKs.
func (o *Options) mapper(ctx context.Context) meta.RESTMapper {
	rm := meta.NewDefaultRESTMapper(sfio.Scheme.PrioritizedVersionsAllGroups())
	for gvk := range sfio.Scheme.AllKnownTypes() {
		rm.Add(gvk, meta.RESTScopeRoot)
	}

	cmd, err := o.Config.Kubectl(ctx, "get", "crds", "--output", "jsonpath", "--template",
		`{range .items[*].spec}{.group}/{.version} {.names.kind} {.names.plural} {.names.singular}{"\n"}{end}`)
	if err != nil {
		return rm
	}

	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return rm
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		f := strings.Fields(scanner.Text())

		gv, err := schema.ParseGroupVersion(f[0])
		if err != nil {
			continue
		}

		rm.AddSpecific(gv.WithKind(f[1]), gv.WithResource(f[2]), gv.WithResource(f[3]), meta.RESTScopeRoot)
	}

	return rm
}

// createRecommendationKustomizePatches constructs patches from recommendation parameters.
func createRecommendationKustomizePatches(mapper meta.RESTMapper, params []applicationsv2.Parameter) ([]types.Patch, error) {
	// Required spec for container resources
	type containerResourceSpec struct {
		Name     string
		Path     string
		Limits   corev1.ResourceList
		Requests corev1.ResourceList
	}

	result := make([]types.Patch, len(params))
	for i := range params {
		// Lookup the GVK from the resource name
		gvks, err := mapper.KindsFor(schema.GroupVersionResource{Resource: params[i].Target.Kind})
		if err != nil {
			return nil, err
		}

		// Start a new patch document
		p := yaml.NewRNode(&yaml.Node{
			Kind:    yaml.DocumentNode,
			Content: []*yaml.Node{{Kind: yaml.MappingNode}},
		})

		// We need to embed the GVK into the patch (see comments for trials)
		apiVersion, kind := gvks[0].ToAPIVersionAndKind()
		if err := p.PipeE(
			yaml.Tee(yaml.SetField("apiVersion", yaml.NewStringRNode(apiVersion))),
			yaml.Tee(yaml.SetField("kind", yaml.NewStringRNode(kind))),
			yaml.Tee(yaml.SetK8sName(params[i].Target.Workload)),
			yaml.Tee(yaml.SetK8sNamespace(params[i].Target.Namespace)),
		); err != nil {
			return nil, err
		}

		for cri := range params[i].ContainerResources {
			// Make the container resources something useful
			spec := containerResourceSpec{}
			if data, err := json.Marshal(params[i].ContainerResources[cri]); err != nil {
				return nil, err
			} else if err := json.Unmarshal(data, &spec); err != nil {
				return nil, err
			}

			// Assume that the spec is not empty (i.e. there are either requests or limits)
			var fns []yaml.Filter

			// Provide a default path if necessary
			if spec.Path == "" {
				switch gvks[0].Kind {
				// TODO ReplicaSet
				case "Deployment", "StatefulSet":
					spec.Path = "/spec/template/spec/containers/[name={ .ContainerName }]/resources"
				default:
					return nil, fmt.Errorf("unable to build patch for %q container resources", params[i].Target.Kind)
				}
			}

			// Parse the field path
			fp, err := sfio.FieldPath(spec.Path, map[string]string{"ContainerName": spec.Name})
			if err != nil {
				return nil, err
			}
			fns = append(fns, yaml.PathGetter{Path: fp, Create: yaml.MappingNode})

			// Add the limits
			if len(spec.Limits) > 0 {
				limits := yaml.NewMapRNode(nil)
				if data, err := json.Marshal(spec.Limits); err != nil {
					return nil, err
				} else if err := yaml.Unmarshal(data, limits.YNode()); err != nil {
					return nil, err
				}
				fns = append(fns, yaml.FieldSetter{Name: "limits", Value: limits})
			}

			// Add the requests
			if len(spec.Requests) > 0 {
				requests := yaml.NewMapRNode(nil)
				if data, err := json.Marshal(spec.Requests); err != nil {
					return nil, err
				} else if err := yaml.Unmarshal(data, requests.YNode()); err != nil {
					return nil, err
				}
				fns = append(fns, yaml.FieldSetter{Name: "requests", Value: requests})
			}

			// Convert container resource into patch...
			if err := p.PipeE(yaml.Tee(fns...)); err != nil {
				return nil, err
			}
		}

		// Encode the result as JSON for the patch
		data, err := p.MarshalJSON()
		if err != nil {
			return nil, err
		}

		// Create the Kustomize patch
		result[i] = types.Patch{
			Patch: string(data),
			Target: &types.Selector{
				KrmId: types.KrmId{
					Gvk:  resid.Gvk{Group: gvks[0].Group, Version: gvks[0].Version, Kind: gvks[0].Kind},
					Name: params[i].Target.Workload,
				},
			},
		}
	}

	return result, nil
}
