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
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commander"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/check"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/initialize"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commands/run/internal"
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

// checkKubectlVersion returns a message containing the kubectl version.
func (o *Options) checkKubectlVersion() tea.Msg {
	ctx := context.TODO()

	cmd, err := o.Config.Kubectl(ctx, "version", "--client", "--output", "json")
	if err != nil {
		return err
	}

	data, err := cmd.Output()
	if err != nil {
		return internal.KubectlVersionMsg("")
	}

	v := struct {
		ClientVersion struct {
			GitVersion string `json:"gitVersion"`
		} `json:"clientVersion"`
	}{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	return internal.KubectlVersionMsg(v.ClientVersion.GitVersion)
}

// checkForgeVersion returns a message containing the forge version.
func (o *Options) checkForgeVersion() tea.Msg {
	ctx := context.TODO()

	cmd := exec.CommandContext(ctx, "forge", "version")

	data, err := cmd.Output()
	if err != nil {
		return internal.ForgeVersionMsg("")
	}

	return internal.ForgeVersionMsg(strings.TrimSpace(string(data)))
}

// checkControllerVersion returns a message containing the controller version.
func (o *Options) checkControllerVersion() tea.Msg {
	ctx := context.TODO()

	v, err := (&versioncmd.Options{Config: o.Config}).ControllerVersion(ctx)
	if err != nil {
		return internal.OptimizeControllerVersionMsg("")
	}

	return internal.OptimizeControllerVersionMsg(v.Version)
}

// checkOptimizeAuthorization returns a message containing the authorization
// status of the Optimize feature.
func (o *Options) checkOptimizeAuthorization() tea.Msg {
	ctx := context.TODO()

	if _, err := o.ExperimentsAPI.Options(ctx); err != nil {
		return internal.OptimizeAuthorizationMsg(internal.AuthorizationInvalid)
	}

	return internal.OptimizeAuthorizationMsg(internal.AuthorizationValid)
}

// checkPerformanceTestAuthorization returns a message containing the authorization
// status of the Performance Test feature.
func (o *Options) checkPerformanceTestAuthorization() tea.Msg {
	ctx := context.TODO()

	ping, err := forge(ctx, "ping")
	if err != nil || ping.Response == nil || ping.Response.Status != "ok" {
		return internal.PerformanceTestAuthorizationMsg(internal.AuthorizationInvalid)
	}

	return internal.PerformanceTestAuthorizationMsg(internal.AuthorizationValid)
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
			IOStreams:            discard,
			IncludeBootstrapRole: true,
			Image:                o.initializationModel.ControllerImage,
		},
		Wait: true,
	}
	if err := initOpts.Initialize(ctx); err != nil {
		return err
	}

	// Now that we are installed, wait for it to become ready again
	if err := checkOpts.CheckController(ctx); err != nil {
		return err
	}

	return o.checkControllerVersion()
}

func (o *Options) listKubernetesNamespaces() tea.Msg {
	ctx := context.TODO()
	msg := internal.KubernetesNamespacesMsg{}

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
	msg := internal.StormForgerTestCasesMsg{}

	orgs, err := forge(ctx, "organization", "list")
	if err != nil {
		return nil
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
	msg := internal.ExperimentMsg{}

	o.Generator.Application.Default()
	o.Generator.SetDefaultSelectors()

	if err := o.Generator.Execute(&msg); err != nil {
		return err
	}

	return msg
}

func (o *Options) createExperimentInCluster() tea.Msg {
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

	return internal.ExperimentCreatedMsg{}
}

func (o *Options) createExperimentOnScreen() tea.Msg {
	content, err := kio.StringAll(o.runModel.experiment)
	if err != nil {
		return err
	}
	o.previewModel.Preview.SetContent(content)
	return nil
}

func (o *Options) refreshTrials() tea.Msg {
	ctx := context.TODO()

	// TODO Where do we get the namespace/selector from?
	namespace := o.previewModel.Experiment.Namespace
	name := o.previewModel.Experiment.Name
	labelSelector := meta.FormatLabelSelector(o.previewModel.Experiment.TrialSelector())

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
			return internal.ExperimentFinishedMsg{}
		case conditionStatus(node, redskyv1beta1.ExperimentFailed) == corev1.ConditionTrue:
			return internal.ExperimentFinishedMsg{Failed: true}
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
	return internal.TrialsMsg(trialNodes)
}

func (o *Options) refreshTrialsTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return internal.TrialsRefreshMsg(t)
	})
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
	Data     []forgeData    `json:"data"`
	Response *forgeResponse `json:"response"`
}
type forgeData struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes forgeAttributes `json:"attributes"`
}
type forgeResponse struct {
	Status string `json:"status"`
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
