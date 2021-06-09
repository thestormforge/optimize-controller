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
	"time"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-go/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/yaml"
)

// ControllerOptions are the options for checking an Optimize controller
type ControllerOptions struct {
	// Config is the Optimize Configuration for connecting to the cluster
	Config *config.OptimizeConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// Wait for the controller to be ready
	Wait bool
}

// NewControllerCommand creates a new command for checking an Optimize controller
func NewControllerCommand(o *ControllerOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Check the controller",
		Long:  "Check the Optimize controller",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.CheckController),
	}

	cmd.Flags().BoolVar(&o.Wait, "wait", o.Wait, "wait for the controller to be ready before returning")

	return cmd
}

func (o *ControllerOptions) CheckController(ctx context.Context) error {
	// Get the namespace and selector
	selector := "control-plane=controller-manager"
	ns, err := o.Config.SystemNamespace()
	if err != nil {
		return err
	}

	// Try to get the pod first; wait will fail if it doesn't exist yet
	var pod *corev1.Pod
	if err := retry.OnError(wait.Backoff{
		Steps:    30,
		Duration: 1 * time.Second,
	}, func(err error) bool {
		// Only retry if we are supposed to be waiting
		return o.Wait
	}, func() error {
		// Get the pod (this is the same query used to fetch the version number)
		get, err := o.Config.Kubectl(ctx, "--namespace", ns, "get", "pods", "--selector", selector, "--ignore-not-found", "--output", "yaml")
		if err != nil {
			return err
		}
		output, err := get.Output()
		if err != nil {
			return fmt.Errorf("could not find controller pods: %w", err)
		}

		list := &corev1.PodList{}
		if err := yaml.Unmarshal(output, list); err != nil {
			return err
		}
		if len(list.Items) == 0 {
			return fmt.Errorf("unable to find controller in namespace '%s'", ns)
		}
		if len(list.Items) > 1 {
			return fmt.Errorf("found multiple controllers in namespace '%s'", ns)
		}

		pod = &list.Items[0]
		return nil
	}); err != nil {
		return err
	}

	// If the pod is ready, we are done
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			_, _ = fmt.Fprintf(o.Out, "Success.\n")
			return nil
		}
	}

	// Check to see if this if there is an issue with pulling the image
	for _, c := range pod.Status.ContainerStatuses {
		if c.State.Waiting != nil && c.State.Waiting.Reason == "ErrImageNeverPull" {
			return fmt.Errorf("controller is not ready, %s", c.State.Waiting.Message)
		}
	}

	// If we aren't waiting, just fail immediately
	if !o.Wait {
		return fmt.Errorf("controller is not ready")
	}

	// Delegate the wait to kubectl
	kubewait, err := o.Config.Kubectl(ctx, "--namespace", ns, "wait", "pods", "--selector", selector, "--for", "condition=Ready=True")
	if err != nil {
		return err
	}

	kubewait.Stdout = ioutil.Discard
	if err := kubewait.Run(); err != nil {
		return fmt.Errorf("could not wait for controller pods: %w", err)
	}

	_, _ = fmt.Fprintf(o.Out, "Success.\n")
	return nil
}
