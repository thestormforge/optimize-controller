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

package version

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/redskyops/redskyops-controller/internal/setup"
	"github.com/redskyops/redskyops-controller/internal/version"
	experimentsv1alpha1 "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/config"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/kustomize"
	"github.com/spf13/cobra"
)

// TODO Add support for getting Red Sky server version
// TODO Add "--client" and "--server" and "--manager" for only printing some versions
// TODO Add a "--notes" option to print the release notes?
// TODO Add an "--output" to control format (json, yaml)
// TODO Check GitHub for new releases
// TODO We should have an option to print setup tools as JSON with the pull policy, e.g. `{"image":"...", "imagePullPolicy":"..."}`...
// TODO Get the Kubernetes version from kubectl?

// defaultTemplate is used to format the version information
const defaultTemplate = `{{range $key, $value := . }}{{$key}} version: {{$value}}
{{end}}`

// Options is the configuration for reporting version information
type Options struct {
	// Config is the Red Sky Configuration
	Config config.Config
	// ExperimentsAPI is used to interact with the Red Sky Experiments API
	ExperimentsAPI experimentsv1alpha1.API
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// Product is the current product name
	Product string
	// ShowSetupToolsImage toggles the setup tools image information
	ShowSetupToolsImage bool
	// ShowControllerImage toggles the controller image information
	ShowControllerImage bool
	// Debug enables error logging
	Debug bool
}

// NewCommand creates a new command for reporting version information
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Long:  "Print the version information for Red Sky Ops components",

		PreRunE: func(cmd *cobra.Command, args []string) error {
			if o.Product == "" {
				o.Product = cmd.Root().Name()
			}

			commander.SetStreams(&o.IOStreams, cmd)

			expAPI, err := commander.NewExperimentsAPI(cmd.Context(), o.Config)
			if err != nil {
				return err
			}

			o.ExperimentsAPI = expAPI

			return nil
		},
		RunE: commander.WithContextE(o.version),
	}

	cmd.Flags().BoolVar(&o.ShowSetupToolsImage, "setuptools-image", false, "Print only the name of the setuptools image.")
	cmd.Flags().BoolVar(&o.ShowControllerImage, "controller-image", false, "Print only the name of the controller image.")
	cmd.Flags().BoolVar(&o.Debug, "debug", o.Debug, "Display debugging information.")

	commander.ExitOnError(cmd)
	return cmd
}

func (o *Options) version(ctx context.Context) error {
	// The show image names need to be printed by themselves
	if o.ShowSetupToolsImage {
		_, _ = fmt.Fprintln(o.Out, setup.Image)
		return nil
	} else if o.ShowControllerImage {
		_, _ = fmt.Fprintln(o.Out, kustomize.BuildImage)
		return nil
	}

	// Collect all the version information into a map
	data := make(map[string]*version.Info, 3)
	if o.Product != "" {
		data[o.Product] = version.GetInfo()
	}
	// TODO Each of these should be done in go routines and time boxed
	if v, err := o.controllerVersion(ctx); err != nil {
		if o.Debug {
			_, _ = fmt.Fprintln(o.ErrOut, "controller:", err.Error())
		}
	} else if v != nil {
		data["controller"] = v
	}
	if v, err := o.apiVersion(ctx); err != nil {
		if o.Debug {
			_, _ = fmt.Fprintln(o.ErrOut, "api:", err.Error())
		}
	} else if v != nil {
		data["api"] = v
	}

	// Format the template using the collected version information
	return template.Must(template.New("version").Parse(defaultTemplate)).Execute(o.Out, data)
}

// controllerVersion looks for the controller pod and executes `/manager version` to extract the version information
func (o *Options) controllerVersion(ctx context.Context) (*version.Info, error) {
	// Get the namespace
	ns, err := o.Config.SystemNamespace()
	if err != nil {
		return nil, err
	}

	// Get the pod name
	get, err := o.Config.Kubectl(ctx, "--namespace", ns, "--request-timeout", "1s", "get", "pods", "--selector", "control-plane=controller-manager", "--output", "name")
	if err != nil {
		return nil, err
	}
	output, err := get.Output()
	if err != nil {
		return nil, err
	}
	podName := strings.TrimSpace(string(output)) // TODO Do we need make sure there was only one?

	// Get the version JSON
	exec, err := o.Config.Kubectl(ctx, "--namespace", ns, "--request-timeout", "1s", "exec", "--container", "manager", podName, "/manager", "version")
	if err != nil {
		return nil, err
	}
	output, err = exec.Output()
	if err != nil {
		return nil, err
	}

	// Unmarshal
	info := &version.Info{}
	if err := json.Unmarshal(output, info); err != nil {
		return nil, err
	}
	return info, nil
}

// apiVersion gets the API server metadata via an HTTP OPTIONS request
func (o *Options) apiVersion(ctx context.Context) (*version.Info, error) {
	// Get the server metadata
	sm, err := o.ExperimentsAPI.Options(ctx)
	if err != nil {
		return nil, err
	}

	// Try to parse out the server header
	var info *version.Info
	parts := strings.SplitN(sm.Server, " ", 2)
	parts = strings.SplitN(parts[0], "/", 2)
	if len(parts) > 1 { // TODO Also check the product name
		info = &version.Info{}
		parts = strings.SplitN(parts[1], "+", 2)
		info.Version = "v" + strings.TrimPrefix(parts[0], "v")
		if len(parts) > 1 {
			info.BuildMetadata = parts[1]
		}
	}
	return info, nil
}
