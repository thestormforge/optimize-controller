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

	"github.com/redskyops/k8s-experiment/pkg/api"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

// TODO Add documentation about what this creates and how it works, including Kustomize support
// TODO How do we collect Red Sky API information? Does it need to be exposed by the cmdutil.Factory?

const (
	initLong    = `Install Red Sky Ops to a cluster`
	initExample = ``
)

func NewInitCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewSetupOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Install to a cluster",
		Long:    initLong,
		Example: initExample,
		Run: func(cmd *cobra.Command, args []string) {
			CheckErr(o.Complete(f, cmd))
			CheckErr(o.Run())
		},
	}

	o.AddFlags(cmd)

	return cmd
}

func (o *SetupOptions) initCluster() error {
	clientConfig, err := api.DefaultConfig()
	if err != nil {
		return err
	}

	bootstrapConfig, err := NewBootstrapInitConfig(o, clientConfig)
	if err != nil {
		return err
	}

	// A bootstrap dry run just means serialize the bootstrap config
	if o.Bootstrap && o.DryRun {
		return bootstrapConfig.Marshal(o.Out)
	}

	// If there are any left over bootstrap objects, remove them before initializing
	deleteFromCluster(bootstrapConfig)

	// If we are bootstrapping the install, leave the objects, otherwise ensure even a partial creation is cleaned up
	if !o.Bootstrap {
		defer deleteFromCluster(bootstrapConfig)
	}

	// Create the bootstrap config to initiate the install job
	if err := createInCluster(bootstrapConfig); err != nil {
		return err
	}

	// When bootstrapping the job won't start so don't wait for it
	if o.Bootstrap {
		return nil
	}

	// Wait for the job to finish
	if err = waitForJob(bootstrapConfig, o.Out, o.ErrOut); err != nil {
		return err
	}

	return nil
}

// The current implementation of Kustomize exec plugins use an executable whose name matches the plugin
// kind and accepts a single argument (the config input file). To support that we create a symlink to the
// `redskyctl` executable from the location Kustomize will invoke it.
func (o *SetupOptions) initKustomize() error {
	e, err := os.Executable()
	if err != nil {
		return err
	}

	p := filepath.Join(kustomizePluginDir()...)
	s := filepath.Join(p, KustomizePluginKind)

	if err = os.MkdirAll(p, 0700); err != nil {
		return err
	}

	if _, err = os.Lstat(s); err == nil {
		if err = os.Remove(s); err != nil {
			return err
		}
	}

	if err = os.Symlink(e, s); err != nil {
		return err
	}

	return nil
}
