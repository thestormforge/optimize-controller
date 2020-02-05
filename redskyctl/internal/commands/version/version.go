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

	"github.com/redskyops/k8s-experiment/internal/config"
	"github.com/redskyops/k8s-experiment/internal/setup"
	"github.com/redskyops/k8s-experiment/pkg/version"
	experimentsv1alpha1 "github.com/redskyops/k8s-experiment/redskyapi/experiments/v1alpha1"
	"github.com/redskyops/k8s-experiment/redskyctl/internal/commander"
	"github.com/spf13/cobra"
)

// TODO Add support for getting Red Sky server version
// TODO Add "--client" and "--server" and "--manager" for only printing some versions
// TODO Add a "--notes" option to print the release notes?
// TODO Add an "--output" to control format (json, yaml)
// TODO Check GitHub for new releases
// TODO We should have an option to print setup tools as JSON with the pull policy, e.g. `{"image":"...", "imagePullPolicy":"..."}`...

// defaultTemplate is used to format the version information
const defaultTemplate = `{{range $key, $value := . }}{{$key}} version: {{$value}}
{{end}}`

type Options struct {
	// Config is the Red Sky Configuration
	Config *config.RedSkyConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// Product is the current product name
	Product string
	// ShowSetupToolsImage toggles the setup tools image information
	ShowSetupToolsImage bool
}

func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Long:  "Print the version information for Red Sky Ops components",

		PreRun: func(cmd *cobra.Command, args []string) {
			commander.SetStreams(&o.IOStreams, cmd)
			if o.Product == "" {
				o.Product = cmd.Root().Name()
			}
			err := o.Complete()
			commander.CheckErr(cmd, err)
		},

		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			commander.CheckErr(cmd, err)
		},
	}

	cmd.Flags().BoolVar(&o.ShowSetupToolsImage, "setuptools", false, "Print only the name of the setuptools image.")

	return cmd
}

func (o *Options) Complete() error {
	return nil
}

func (o *Options) Run() error {
	// The setup tools image name needs to be printed by itself
	if o.ShowSetupToolsImage {
		_, _ = fmt.Fprintln(o.Out, setup.Image)
		return nil
	}

	// Collect all the version information into a map
	data := make(map[string]*version.Info, 3)
	if o.Product != "" {
		data[o.Product] = version.GetInfo()
	}
	// TODO Each of these should be done in go routines and time boxed
	if v, err := o.controllerVersion(); err == nil && v != nil {
		data["controller"] = v
	}
	if v, err := o.apiVersion(); err == nil && v != nil {
		data["api"] = v
	}

	// Format the template using the collected version information
	return template.Must(template.New("version").Parse(defaultTemplate)).Execute(o.Out, data)
}

// controllerVersion looks for the controller pod and executes `/manager version` to extract the version information
func (o *Options) controllerVersion() (*version.Info, error) {
	// Get the namespace
	ns, err := o.Config.SystemNamespace()
	if err != nil {
		return nil, err
	}

	// Get the pod name
	get, err := o.Config.Kubectl("--namespace", ns, "--request-timeout", "1s", "get", "pods", "--selector", "control-plane=controller-manager", "--output", "name")
	if err != nil {
		return nil, err
	}
	output, err := get.Output()
	if err != nil {
		return nil, err
	}
	podName := strings.TrimSpace(string(output)) // TODO Do we need make sure there was only one?

	// Get the version JSON
	exec, err := o.Config.Kubectl("--namespace", ns, "--request-timeout", "1s", "exec", "--container", "manager", podName, "/manager", "version")
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

func (o *Options) apiVersion() (*version.Info, error) {
	// Get an API
	api, err := experimentsv1alpha1.NewForConfig(o.Config, version.UserAgent("redskyctl", nil))
	if err != nil {
		return nil, err
	}

	// Get the server metadata
	sm, err := api.Options(context.Background())
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
