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

package generate

import (
	"context"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/thestormforge/konjure/pkg/konjure"
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/internal/experiment"
	"github.com/thestormforge/optimize-go/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
)

type ExperimentOptions struct {
	// Config is the Optimize Configuration used to generate the experiment
	Config *config.OptimizeConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	Generator experiment.Generator

	Filename  string
	Resources []string
}

// Other possible options:
// Have the option to create (or not create?) untracked metrics (e.g. CPU and Memory requests along side Cost)

func NewExperimentCommand(o *ExperimentOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "experiment",
		Aliases: []string{"exp"},
		Short:   "Generate an experiment",
		Long:    "Generate an experiment from an application descriptor",

		PreRun: func(cmd *cobra.Command, args []string) {
			commander.SetStreams(&o.IOStreams, cmd)
			o.Generator.DefaultReader = cmd.InOrStdin()
		},
		RunE: commander.WithoutArgsE(o.generate),
	}

	cmd.Flags().StringVarP(&o.Filename, "filename", "f", o.Filename, "file that contains the application definition")
	cmd.Flags().StringArrayVarP(&o.Resources, "resources", "r", nil, "additional resources to consider")
	cmd.Flags().StringVar(&o.Generator.ExperimentName, "name", o.Generator.ExperimentName, "override the experiment `name`")
	cmd.Flags().StringVarP(&o.Generator.Scenario, "scenario", "s", o.Generator.Scenario, "the application scenario to generate an experiment for")
	cmd.Flags().StringVar(&o.Generator.Objective, "objective", o.Generator.Objective, "the application objective to generate an experiment for")
	cmd.Flags().BoolVar(&o.Generator.IncludeApplicationResources, "include-resources", false, "include the application resources in the output")

	_ = cmd.MarkFlagFilename("filename", "yml", "yaml")

	return cmd
}

func (o *ExperimentOptions) generate() error {
	if o.Filename != "" {
		r, err := o.IOStreams.OpenFile(o.Filename)
		if err != nil {
			return err
		}

		rr := commander.NewResourceReader()
		if err := rr.ReadInto(r, &o.Generator.Application); err != nil {
			return err
		}
	}

	if err := o.filterResources(&o.Generator.Application); err != nil {
		return err
	}

	// Make sure we have a path on the application to use as a base for resolving relative file paths
	if err := o.setPath(); err != nil {
		return err
	}

	// Make sure there is an explicit namespace and name
	if o.Generator.Application.Namespace == "" {
		o.Generator.Application.Namespace = o.defaultNamespace()
	}
	if o.Generator.Application.Name == "" {
		o.Generator.Application.Name = o.defaultName()
	}

	// Generate the experiment
	return o.Generator.Execute(o.YAMLWriter())
}

func (o *ExperimentOptions) filterResources(app *optimizeappsv1alpha1.Application) error {
	// Add additional resources (this allows addition manifests to be added when invoking the CLI)
	if len(o.Resources) > 0 {
		app.Resources = append(app.Resources, konjure.NewResource(o.Resources...))
	}

	// If there are no resources, assume the directory of the input file (or "." if no file is specified)
	if len(app.Resources) == 0 {
		app.Resources = append(app.Resources, konjure.NewResource(filepath.Dir(o.Filename)))
	}

	return nil
}

func (o *ExperimentOptions) setPath() (err error) {
	path := o.Filename
	if path == "" || path == "-" || filepath.Dir(path) == "/dev/fd" {
		// The filename here could be anything, the important part is the directory so
		// when you do not supply `-f`, relative paths resolve against the working directory
		path = "app.yaml"
	}

	// Ensure the path is absolute, relative to the current working directory
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}

	metav1.SetMetaDataAnnotation(&o.Generator.Application.ObjectMeta, kioutil.PathAnnotation, path)
	return nil
}

func (o *ExperimentOptions) defaultNamespace() string {
	// First check to see if we have an explicit namespace override set
	if cstr, err := config.CurrentCluster(o.Config.Reader()); err == nil && cstr.Namespace != "" {
		return cstr.Namespace
	}

	// Check the kubectl config output
	cmd, err := o.Config.Kubectl(context.Background(), "config", "view", "--minify", "-o", "jsonpath='{.contexts[0].context.namespace}'")
	if err == nil {
		if out, err := cmd.CombinedOutput(); err == nil {
			if ns := strings.TrimSpace(strings.Trim(string(out), "'")); ns != "" {
				return ns
			}
		}
	}

	// We cannot return empty, use default
	return "default"
}

func (o *ExperimentOptions) defaultName() string {
	var d, f = o.Filename, ""
	for {
		// Split the path, discard everything if this was a file descriptor path
		d, f = filepath.Split(d)
		if d == "/dev/fd/" {
			d, f = "", ""
		}

		// Trim the first extension
		f = strings.TrimSuffix(f, filepath.Ext(f))

		// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
		f = strings.Map(dnsSubdomainChars, f)
		f = strings.Trim(f, "-.")
		if len(f) > 253 {
			f = f[0:253]
		}

		// If the name is good, return it
		if f != "" && f != "application" && f != "app" {
			return f
		}

		// Ensure the directory is an absolute path for remaining iterations
		ad, err := filepath.Abs(d)
		if err != nil || d == "/" {
			return "default"
		}

		// Walk up the tree
		d = ad
	}
}

func dnsSubdomainChars(r rune) rune {
	if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
		return r
	}
	if r >= 'A' && r <= 'Z' {
		return unicode.ToLower(r)
	}
	return -1
}
