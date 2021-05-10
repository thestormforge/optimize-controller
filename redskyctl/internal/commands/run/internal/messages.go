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

package internal

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// Error creates a command wrapping the supplied error. This is useful for some
// update implementations that may need to check errors.
func Error(err error) tea.Cmd { return func() tea.Msg { return err } }

// InitializationFinished indicates that all initialization tasks have completed.
type InitializationFinished struct{}

// KubectlVersionMsg carries the version of the kubectl executable. If the
// executable is not on the path, the message will be an empty string.
type KubectlVersionMsg string

// ForgeVersionMsg carries the version of the forge executable. If the
// executable is not on the path, the message will be an empty string.
type ForgeVersionMsg string

// OptimizeControllerVersionMsg carries the version of the Optimize Controller
// in the cluster. If the controller is not installed or the cluster cannot be
// reached, the message will be an empty string.
type OptimizeControllerVersionMsg string

// OptimizeAuthorizationMsg is used to notify of changes to the authorization
// status of the Optimize API.
type OptimizeAuthorizationMsg AuthorizationStatus

// PerformanceTestAuthorizationMsg is used to notify of changes to the authorization
// status of the Performance Test API.
type PerformanceTestAuthorizationMsg AuthorizationStatus

// KubernetesNamespacesMsg is used to report the list of available Kubernetes
// namespaces.
type KubernetesNamespacesMsg []string

// StormForgerTestCasesMsg is used to report the list of available StormForger
// test case names.
type StormForgerTestCasesMsg []string

// ExperimentMsg represents the generated experiment.
type ExperimentMsg []*yaml.RNode

// Write allows the experiment message to be populated using a KYAML pipeline.
func (m *ExperimentMsg) Write(nodes []*yaml.RNode) error {
	*m = nodes
	return nil
}

// ExperimentReadyMsg is used to indicate that the experiment is ready and user
// would like it sent to a specific destination.
type ExperimentReadyMsg struct {
	Cluster bool
	File    bool
}

// ExperimentCreatedMsg indicates that the experiment has been successfully
// applied to a cluster or file.
type ExperimentCreatedMsg struct {
	Filename string
}

// ExperimentFinishedMsg indicates that the experiment has completed or failed.
type ExperimentFinishedMsg struct {
	Failed bool
}

// TrialsMsg represents the current trial list of the experiment fetched as part
// of a status update.
type TrialsMsg []*yaml.RNode

// TrialsRefreshMsg is used to indicate when the list of trials should be refreshed.
type TrialsRefreshMsg time.Time
