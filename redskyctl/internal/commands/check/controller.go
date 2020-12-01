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

package check

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commander"
	"github.com/thestormforge/optimize-go/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// ControllerOptions are the options for checking a Red Sky controller
type ControllerOptions struct {
	// Config is the Red Sky Configuration for connecting to the cluster
	Config *config.RedSkyConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// Wait for the controller to be ready
	Wait bool
}

// NewControllerCommand creates a new command for checking a Red Sky controller
func NewControllerCommand(o *ControllerOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Check the controller",
		Long:  "Check the Red Sky controller",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.checkController),
	}

	cmd.Flags().BoolVar(&o.Wait, "wait", o.Wait, "wait for the controller to be ready before returning")

	return cmd
}

func (o *ControllerOptions) checkController(ctx context.Context) error {
	// Get the namespace
	ns, err := o.Config.SystemNamespace()
	if err != nil {
		return err
	}

	// Delegate the wait to kubectl
	if o.Wait {
		wait, err := o.Config.Kubectl(ctx, "--namespace", ns, "wait", "pods", "--selector", "control-plane=controller-manager", "--for", "condition=Ready=True")
		if err != nil {
			return err
		}
		wait.Stdout = ioutil.Discard
		if err := wait.Run(); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(o.Out, "Success.\n")
		return nil
	}

	// Get the pod (this is the same query used to fetch the version number)
	get, err := o.Config.Kubectl(ctx, "--namespace", ns, "get", "pods", "--selector", "control-plane=controller-manager", "--output", "yaml")
	if err != nil {
		return err
	}
	output, err := get.Output()
	if err != nil {
		return err
	}

	// For this check we are just going to assume it is safe to deserialize into a v1 PodList
	list := &corev1.PodList{}
	if err := yaml.Unmarshal(output, list); err != nil {
		return err
	}

	// We are expecting a single item list
	if len(list.Items) == 0 {
		return fmt.Errorf("unable to find controller in namespace '%s'", ns)
	}
	if len(list.Items) > 1 {
		return fmt.Errorf("found multiple controllers in namespace '%s'", ns)
	}
	pod := &list.Items[0]

	// Check to see if the pod is ready
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status != corev1.ConditionTrue {
			return fmt.Errorf("controller is not ready")
		}
	}

	_, _ = fmt.Fprintf(o.Out, "Success.\n")
	return nil
}
