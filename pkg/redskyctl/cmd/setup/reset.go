/*
Copyright 2019 GramLabs, Inc.

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
package setup

import (
	"os"
	"path/filepath"

	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	resetLong    = `The reset command will uninstall the Red Sky manifests.`
	resetExample = ``
)

func NewResetCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewSetupOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "reset",
		Short:   "Uninstall Red Sky from a cluster",
		Long:    resetLong,
		Example: resetExample,
		Run: func(cmd *cobra.Command, args []string) {
			CheckErr(o.Complete(f, cmd))
			CheckErr(o.Run())
		},
	}

	o.AddFlags(cmd)

	return cmd
}

func (o *SetupOptions) resetCluster() error {
	bootstrapConfig, err := NewBootstrapResetConfig(o)
	if err != nil {
		return err
	}

	// A bootstrap dry run just means serialize the bootstrap config
	if o.Bootstrap && o.DryRun {
		return bootstrapConfig.Marshal(o.Out)
	}

	// Remove bootstrap objects and return if that was all we are doing
	deleteFromCluster(bootstrapConfig)
	if o.Bootstrap {
		return nil
	}

	// Ensure a partial bootstrap is cleaned up properly
	defer deleteFromCluster(bootstrapConfig)

	// Create the bootstrap config to initiate the uninstall job
	podWatch, err := createInCluster(bootstrapConfig)
	if podWatch != nil {
		defer podWatch.Stop()
	}
	if err != nil {
		return err
	}

	// Wait for the job to finish; ignore errors as we are having the namespace pulled out from under us
	_ = waitForJob(o.ClientSet.CoreV1().Pods(o.namespace), podWatch, nil, nil)

	return nil

}

func (o *SetupOptions) resetKustomize() error {
	// TODO Walk back through the array to clean up empty directories
	p := filepath.Join(kustomizePluginDir()...)
	return os.RemoveAll(p)
}
