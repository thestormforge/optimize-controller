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

package generate

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	konjurev1beta2 "github.com/thestormforge/konjure/pkg/api/core/v1beta2"
	"github.com/thestormforge/konjure/pkg/konjure"
	"github.com/thestormforge/optimize-controller/internal/application"
	"github.com/thestormforge/optimize-controller/redskyctl/internal/commander"
	"github.com/thestormforge/optimize-go/pkg/config"
	"sigs.k8s.io/kustomize/kyaml/kio"
)

type ApplicationOptions struct {
	// Config is the Red Sky Configuration used to generate the application
	Config *config.RedSkyConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	Generator       application.Generator
	Resources       []string
	DefaultResource konjurev1beta2.Kubernetes
}

func NewApplicationCommand(o *ApplicationOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "application",
		Short: "Generate an application",
		Long:  "Generate an application descriptor",

		PreRun: func(cmd *cobra.Command, args []string) {
			commander.SetStreams(&o.IOStreams, cmd)
			o.Generator.DefaultReader = cmd.InOrStdin()
		},
		RunE: commander.WithoutArgsE(o.generate),
	}

	cmd.Flags().StringVar(&o.Generator.Name, "name", "", "set the application `name`")
	cmd.Flags().StringSliceVar(&o.Generator.Objectives, "objectives", []string{"p95-latency", "cost"}, "specify the application optimization `obj`ectives")
	cmd.Flags().BoolVar(&o.Generator.Documentation.Disabled, "no-comments", false, "suppress documentation comments on output")
	cmd.Flags().StringArrayVarP(&o.Resources, "resources", "r", nil, "additional resources to consider")
	cmd.Flags().StringArrayVar(&o.DefaultResource.Namespaces, "namespace", nil, "select resources from a specific namespace")
	cmd.Flags().StringVar(&o.DefaultResource.NamespaceSelector, "ns-selector", "", "`sel`ect resources from labeled namespaces")
	cmd.Flags().StringVarP(&o.DefaultResource.LabelSelector, "selector", "l", "", "`sel`ect only labeled resources")

	return cmd
}

func (o *ApplicationOptions) generate() error {
	if len(o.Resources) > 0 {
		// Add explicitly requested resources
		o.Generator.Resources = append(o.Generator.Resources, konjure.NewResource(o.Resources...))
	} else if o.DefaultResource.LabelSelector == "" && o.Generator.Name != "" {
		// Add a default the label selector based on the name
		o.DefaultResource.LabelSelector = "app.kubernetes.io/name=" + o.Generator.Name
	}

	// Only include the default resource if it has values
	if !o.isDefaultResourceEmpty() {
		o.Generator.Resources = append(o.Generator.Resources, konjure.Resource{Kubernetes: &o.DefaultResource})
	}

	// Prefer Git resources if possible, this will make the resulting application more portable
	// Note that using Git will be slower then if we just referenced the file system directly
	o.preferGit()

	// Generate the application
	return o.Generator.Execute(&kio.ByteWriter{Writer: o.Out})
}

func (o *ApplicationOptions) isDefaultResourceEmpty() bool {
	// TODO Should we check that `kubectl` is available and can return something meaningful?
	return len(o.DefaultResource.Namespaces) == 0 &&
		o.DefaultResource.NamespaceSelector == "" &&
		len(o.DefaultResource.Types) == 0 &&
		o.DefaultResource.LabelSelector == ""
}

func (o *ApplicationOptions) preferGit() {
	var resources []konjure.Resource
	for _, r := range o.Generator.Resources {
		switch {
		case r.Resource != nil:
			var resourceSpecs []string
			for _, rr := range r.Resource.Resources {
				if gr := o.asGitResource(rr); gr != nil {
					resources = append(resources, konjure.Resource{Git: gr})
					continue
				}

				resourceSpecs = append(resourceSpecs, rr)
			}
			if len(resourceSpecs) > 0 {
				r.Resource.Resources = resourceSpecs
				resources = append(resources, r)
			}

		default:
			resources = append(resources, r)
		}

	}
	o.Generator.Resources = resources
}

// asGitResource tests to see if the supplied path points to a Git resource. Git reference
// will be more portable when recorded in an app.yaml file because they are not specific to
// the current workstation.
func (o *ApplicationOptions) asGitResource(path string) *konjurev1beta2.Git {
	// These will fail in stat, may as well just save the call
	for _, prefix := range []string{"http://", "https://", "git::", "git@"} {
		if strings.HasPrefix(path, prefix) {
			return nil
		}
	}

	var dir, file = path, ""
	if fi, err := os.Stat(dir); err != nil {
		return nil
	} else if !fi.IsDir() {
		dir, file = filepath.Split(dir)
	}

	// Assume we only want this to work with origin
	remote := runGit(dir, "remote", "get-url", "origin")
	if len(remote) != 1 {
		return nil
	}

	revParse := runGit(dir, "rev-parse", "--show-prefix", "--abbrev-ref", "HEAD")
	if len(revParse) != 2 {
		return nil
	}
	if revParse[1] == "master" {
		revParse[1] = ""
	}

	return &konjurev1beta2.Git{
		Repository: remote[0],
		Refspec:    revParse[1],
		Context:    revParse[0] + file,
	}
}

func runGit(dir string, args ...string) []string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}
