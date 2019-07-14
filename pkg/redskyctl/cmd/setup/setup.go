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
	"fmt"
	"os"
	"strings"

	cmdutil "github.com/gramLabs/redsky/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

const (
	KustomizePluginKind = "ExperimentGenerator"
)

type SetupError struct {
	ImagePullBackOff string
	PodDeleted       bool
}

func (e *SetupError) Error() string {
	switch {
	case e.ImagePullBackOff != "":
		return fmt.Sprintf("unable to pull image '%s'", e.ImagePullBackOff)
	case e.PodDeleted:
		return "setup pod was deleted before it could finish"
	}
	return "encountered an error"
}

type SetupOptions struct {
	Bootstrap bool
	DryRun    bool
	Kustomize bool

	namespace string
	name      string

	ClientSet *kubernetes.Clientset

	Run func() error
	cmdutil.IOStreams
}

func NewSetupOptions(ioStreams cmdutil.IOStreams) *SetupOptions {
	return &SetupOptions{
		namespace: "redsky-system",
		name:      "redsky-bootstrap",
		IOStreams: ioStreams,
	}
}

func (o *SetupOptions) AddFlags(cmd *cobra.Command) {
	// TODO Adjust usage strings based on `cmd.Name()`
	cmd.Flags().BoolVar(&o.Bootstrap, "bootstrap", false, "stop after creating the bootstrap configuration")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "generate the manifests instead of applying them")
	cmd.Flags().BoolVar(&o.Kustomize, "kustomize", false, "install/update the Kustomize plugin and exit")
}

func (o *SetupOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	var err error
	o.ClientSet, err = f.KubernetesClientSet()
	if err != nil {
		return err
	}

	switch cmd.Name() {
	case "init":
		if o.Kustomize {
			o.Run = o.initKustomize
		} else {
			o.Run = o.initCluster
		}
	case "reset":
		if o.Kustomize {
			o.Run = o.resetKustomize
		} else {
			o.Run = o.resetCluster
		}
	default:
		o.Run = func() error { panic("invalid command for setup: " + cmd.Name()) }
	}

	return nil
}

func CheckErr(err error) {
	// Add some special handling for setup errors
	if e, ok := err.(*SetupError); ok {
		_, _ = fmt.Fprintf(os.Stderr, "Failed: %s\n", err.Error())
		if e.ImagePullBackOff != "" {
			if strings.HasPrefix(e.ImagePullBackOff, "gcr.io/carbon-relay-dev/") {
				_, _ = fmt.Fprintf(os.Stderr, "  The image '%s' appears to be a development image, did you remember to configure image pull secrets?", e.ImagePullBackOff)
			} else if !strings.Contains(e.ImagePullBackOff, "/") {
				_, _ = fmt.Fprintf(os.Stderr, "  The image '%s' appears to be a local development image, did you remember to build the image?", e.ImagePullBackOff)
			}
		}
		os.Exit(1)
	}

	cmdutil.CheckErr(err)
}
