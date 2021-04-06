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

package run

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	konjurev1beta2 "github.com/thestormforge/konjure/pkg/api/core/v1beta2"
	"github.com/thestormforge/konjure/pkg/konjure"
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/version"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commander"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/check"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/initialize"
	versioncmd "github.com/thestormforge/optimize-controller/redskyctl/internal/commands/version"
	"github.com/thestormforge/optimize-go/pkg/config"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// This is where you will find all of the tea.Cmd functions that are used to
// perform (potentially blocking) I/O. It is important to isolate the
// the functionality so it can run asynchronously to main event loop (thus
// keeping the TUI responsive).

func (o *Options) checkBuildVersion() tea.Msg {
	return versionMsg{Build: *version.GetInfo()}
}

func (o *Options) checkKubectlVersion() tea.Msg {
	ctx := context.TODO()
	msg := versionMsg{}
	msg.Kubectl.ClientVersion.GitVersion = unknownVersion

	cmd, err := o.Config.Kubectl(ctx, "version", "--client", "--output", "json")
	if err != nil {
		return err
	}
	data, err := cmd.Output()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &msg.Kubectl); err != nil {
		return err
	}

	return msg
}

func (o *Options) checkControllerVersion() tea.Msg {
	ctx := context.TODO()
	msg := versionMsg{}
	msg.Controller.Version = unknownVersion

	if v, err := (&versioncmd.Options{Config: o.Config}).ControllerVersion(ctx); err == nil {
		msg.Controller = *v
	}

	return msg
}

func (o *Options) checkForgeVersion() tea.Msg {
	ctx := context.TODO()
	msg := versionMsg{}
	msg.Forge = unknownVersion

	cmd := exec.CommandContext(ctx, "forge", "version")
	data, err := cmd.Output()
	if err != nil {
		return msg // Ignore the error, leave version "unknown"
	}
	msg.Forge = strings.TrimSpace(string(data))

	return msg
}

func (o *Options) checkAuthorization() tea.Msg {
	ctx := context.TODO()
	msg := azValid

	if _, err := o.ExperimentsAPI.Options(ctx); err != nil {
		msg = azInvalid
	}

	return msg
}

func (o *Options) initializeController() tea.Msg {
	ctx := context.TODO()

	// Wait for the controller to become ready if it is installed
	checkOpts := check.ControllerOptions{
		Config:    o.Config,
		IOStreams: discard,
	}
	if err := checkOpts.CheckController(ctx); err == nil {
		return o.checkControllerVersion()
	}

	// Error indicates the controller is not yet installed
	initOpts := &initialize.Options{
		GeneratorOptions: initialize.GeneratorOptions{
			Config:               o.Config,
			IOStreams:            discard, // TODO The writer should bump the progress
			IncludeBootstrapRole: true,
			Image:                o.welcomeModel.ControllerImage,
		},
		Wait: true,
	}
	if err := initOpts.Initialize(ctx); err != nil {
		return err
	}

	// TODO This should be happening asynchronously
	o.welcomeModel.InitializationPercent = 1.0

	// Now that we are installed, wait for it to become ready again
	if err := checkOpts.CheckController(ctx); err != nil {
		return err
	}

	return o.checkControllerVersion()
}

func (o *Options) listKubernetesNamespaces() tea.Msg {
	ctx := context.TODO()
	msg := kubernetesNamespaceMsg{}

	cmd, err := o.Config.Kubectl(ctx, "get", "namespaces", "--output", "name")
	if err != nil {
		return err
	}
	data, err := cmd.Output()
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		if ns := strings.TrimPrefix(scanner.Text(), "namespace/"); !o.hideKubernetesNamespace(ns) {
			msg = append(msg, ns)
		}
	}

	return msg
}

func (o *Options) listStormForgerTestCaseNames() tea.Msg {
	ctx := context.TODO()
	msg := stormForgerTestCaseMsg{}

	orgs, err := forge(ctx, "organization", "list")
	if err != nil {
		return err
	}

	for i := range orgs.Data {
		org := orgs.Data[i].Attributes.Name
		testCases, err := forge(ctx, "test-case", "list", org)
		if err != nil {
			return err
		}
		for j := range testCases.Data {
			testCase := testCases.Data[j].Attributes.Name
			msg = append(msg, fmt.Sprintf("%s/%s", org, testCase))
		}
	}

	return msg
}

func (o *Options) generateExperiment() tea.Msg {
	msg := experimentMsg{}

	// TODO DEMO ONLY HACK
	if namespaces := o.generationModel.NamespaceInput.Values(); len(namespaces) == 1 {
		o.Generator.Application.Name = namespaces[0]
		o.Generator.Application.Namespace = namespaces[0]
	}

	o.Generator.SetDefaultSelectors()
	o.Generator.Application.Default()
	if err := o.Generator.Execute(kio.WriterFunc(func(nodes []*yaml.RNode) error {
		msg = nodes
		return nil
	})); err != nil {
		if o.Debug {
			if f, err := os.OpenFile(".debug.app.yaml", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600); err == nil {
				defer f.Close()
				_ = json.NewEncoder(f).Encode(&o.Generator.Application)
			}
		}

		return err
	}

	return msg
}

func (o *Options) createExperiment() tea.Msg {
	ctx := context.TODO()

	data, err := kio.StringAll(o.runModel.experiment)
	if err != nil {
		return err
	}

	cmd, err := o.Config.Kubectl(ctx, "create", "-f", "-")
	if err != nil {
		return err
	}

	cmd.Stdin = strings.NewReader(data)
	if _, err := cmd.Output(); err != nil {
		return fmt.Errorf("could not create experiment, %w", err)
	}

	return expCreated
}

func (o *Options) refreshExperimentStatus() tea.Msg {
	ctx := context.TODO()

	// TODO Where do we get the namespace/selector from?
	namespace := o.previewModel.experiment.Namespace
	name := o.previewModel.experiment.Name
	labelSelector := meta.FormatLabelSelector(o.previewModel.experiment.TrialSelector())

	getExperiment, err := o.Config.Kubectl(ctx,
		"get", "experiment",
		"--namespace", namespace,
		name,
		"--output", "yaml")
	if err != nil {
		return err
	}

	expNodes, err := (*execReader)(getExperiment).Read()
	if err != nil {
		return fmt.Errorf("could not get experiment for status, %w", err)
	}
	for _, node := range expNodes {
		switch {
		case conditionStatus(node, redskyv1beta1.ExperimentComplete) == corev1.ConditionTrue:
			return expCompleted
		case conditionStatus(node, redskyv1beta1.ExperimentFailed) == corev1.ConditionTrue:
			return expFailed
		}
	}

	getTrials, err := o.Config.Kubectl(ctx,
		"get", "trials",
		"--namespace", namespace,
		"--selector", labelSelector,
		"--output", "yaml")
	if err != nil {
		return err
	}

	trialNodes, err := (*execReader)(getTrials).Read()
	if err != nil {
		return fmt.Errorf("could not get trials for status, %w", err)
	}
	return trialsMsg(trialNodes)
}

func (o *Options) refreshTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return refreshStatusMsg(t)
	})
}

func (m *generationModel) generateResources() tea.Msg {
	msg := resourceMsg{}

	switch m.ResourceMode {
	case resourceNamespace:
		for i, ns := range m.NamespaceInput.Values() {
			msg = append(msg, konjure.Resource{
				Kubernetes: &konjurev1beta2.Kubernetes{
					Namespaces: []string{ns},
					Selector:   m.LabelSelectorInputs[i].Value(),
				},
			})
		}
	}

	return msg
}

func (m *generationModel) generateIngress() tea.Msg {
	msg := ingressMsg{}

	if u := m.IngressURLInput.Value(); u != "" {
		msg.URL = u
	} else {
		return nil
	}

	return msg
}

func (m *generationModel) generateScenarios() tea.Msg {
	msg := scenarioMsg{}

	switch m.ScenarioMode {
	case scenarioStormForger:
		msg = append(msg, redskyappsv1alpha1.Scenario{
			StormForger: &redskyappsv1alpha1.StormForgerScenario{
				TestCase: m.TestCaseInput.Value(),
			},
		})

	case scenarioLocust:
		msg = append(msg, redskyappsv1alpha1.Scenario{
			Name: m.LocustNameInput.Value(),
			Locust: &redskyappsv1alpha1.LocustScenario{
				Locustfile: m.LocustfileInput.Value(),
			},
		})
	}

	return msg
}

func (m *generationModel) generateParameters() tea.Msg {
	msg := parameterMsg{}

	if sel := m.ContainerResourcesSelectorInput.Value(); sel != "" {
		msg = append(msg, redskyappsv1alpha1.Parameter{
			ContainerResources: &redskyappsv1alpha1.ContainerResources{
				Selector: sel,
			},
		})
	}

	if sel := m.ReplicasSelectorInput.Value(); sel != "" {
		msg = append(msg, redskyappsv1alpha1.Parameter{
			Replicas: &redskyappsv1alpha1.Replicas{
				Selector: sel,
			},
		})
	}

	return msg
}

func (m *generationModel) generateObjectives() tea.Msg {
	msg := objectiveMsg{}

	msg = append(msg, redskyappsv1alpha1.Objective{})
	for _, goal := range m.ObjectiveInput.Values() {
		msg[0].Goals = append(msg[0].Goals, redskyappsv1alpha1.Goal{Name: goal})
	}

	return msg
}

// =============================================================================
// All the tea.Cmd functions are above, helpers are below
// =============================================================================

// discard is an IOStreams equivalent of ioutil.Discard for combined output.
var discard = commander.IOStreams{
	Out:    ioutil.Discard,
	ErrOut: ioutil.Discard,
}

// hideKubernetesNamespace is used to filter the list of namespaces to display
func (o *Options) hideKubernetesNamespace(ns string) bool {
	// Take care of some hardcoded defaults
	switch ns {
	case "kube-system", "kube-node-lease", "kube-public":
		return true
	}

	// Don't show the currently configured namespace for the controller
	if ctrl, err := config.CurrentController(o.Config.Reader()); err == nil && ctrl.Namespace == ns {
		return true
	}

	return false
}

// forge invokes the StormForger CLI with the supplied arguments.
func forge(ctx context.Context, args ...string) (*forgePayload, error) {
	cmd := exec.CommandContext(ctx, "forge", append(args, "--output", "json")...)
	data, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	payload := &forgePayload{}
	if err := json.Unmarshal(data, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

// forgePayload represents envelope for forge responses.
type forgePayload struct {
	Data []forgeData `json:"data"`
}
type forgeData struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes forgeAttributes `json:"attributes"`
}
type forgeAttributes struct {
	Name string `json:"name"`
}

// execReader is used to parse YAML output from a command.
type execReader exec.Cmd

func (r *execReader) Read() ([]*yaml.RNode, error) {
	data, err := (*exec.Cmd)(r).Output()
	if err != nil {
		return nil, err
	}
	return kio.FromBytes(data)
}

// conditionStatus looks for experiment conditions given a YAML representation of the experiment.
func conditionStatus(n *yaml.RNode, t redskyv1beta1.ExperimentConditionType) corev1.ConditionStatus {
	v, err := n.Pipe(yaml.Lookup("status", "conditions", fmt.Sprintf("[type=%s]", t), "status"))
	if err == nil && v != nil {
		return corev1.ConditionStatus(v.YNode().Value)
	}
	return corev1.ConditionUnknown
}
