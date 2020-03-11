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

package kustomize

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/kustomize/consts"
	"github.com/spf13/cobra"
)

// ConfigOptions are the options for configuring a Kustomization
type ConfigOptions struct {
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	Kustomize string
	Filename  string
}

// NewConfigCommand creates a new command for configuring a Kustomization
func NewConfigCommand(o *ConfigOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configure Kustomize transformers",
		Long:  "Configure Kustomize transformers for Red Sky types",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithoutArgsE(o.config),
	}

	cmd.Flags().StringVarP(&o.Kustomize, "kustomize", "k", o.Kustomize, "Kustomize `root` to update")
	cmd.Flags().StringVarP(&o.Filename, "filename", "f", o.Filename, "`file` to write the configuration to (relative to the Kustomize root, if specified)")

	commander.ExitOnError(cmd)
	return cmd
}

func (o *ConfigOptions) config() error {
	// If a Kustomization root is specified, normalize the file paths
	if o.Kustomize != "" {
		// Adjust the value to point at the kustomization file
		var err error
		if o.Kustomize, err = kustomizationFilename(o.Kustomize); err != nil {
			return err
		}

		// Adjust the filename to point to where our configuration should go
		root := filepath.Dir(o.Kustomize)
		if o.Filename == "" {
			o.Filename = filepath.Join(root, "kustomizeconfig", "redskyops.yaml")
		} else if filepath.IsAbs(o.Filename) {
			if rel, err := filepath.Rel(root, o.Filename); err != nil || rel == o.Filename {
				return fmt.Errorf("filename must relative or inside the Kustomization root")
			}
		} else {
			o.Filename = filepath.Join(root, o.Filename)
		}

		// Make sure the directory for the configuration file exists
		if err := os.MkdirAll(filepath.Dir(o.Filename), 0755); err != nil {
			return err
		}
	}

	// If there is no file name, just dump to the output stream
	if o.Filename == "" {
		_, err := o.Out.Write(consts.GetRedSkyFieldSpecs())
		return err
	}

	// Write the file
	if err := ioutil.WriteFile(o.Filename, consts.GetRedSkyFieldSpecs(), 0644); err != nil {
		return err
	}

	// Configure the kustomization
	if o.Kustomize != "" {
		// TODO Apply YAML quoting if necessary?
		path, err := filepath.Rel(filepath.Dir(o.Kustomize), o.Filename)
		if err != nil {
			return err
		}

		// There are no edit commands to add a configuration to a kustomization file, try it the hard way
		// TODO We don't detect duplicates using this method...
		r := regexp.MustCompile(`(?m)^configurations:\s*^(\s*)-`)
		kustfile, err := ioutil.ReadFile(o.Kustomize)
		if err != nil {
			return err
		}
		if r.Match(kustfile) {
			kustfile = r.ReplaceAll(kustfile, []byte("configurations:\n$1- "+path+"\n$1-"))
		} else {
			kustfile = append(kustfile, []byte(fmt.Sprintf("\nconfigurations:\n- %s\n", path))...)
		}
		if err := ioutil.WriteFile(o.Kustomize, kustfile, 0644); err != nil {
			return err
		}
	}

	return nil
}

func isRecognizedKustomizationFilename(f string) bool {
	return f == "kustomization.yaml" || f == "kustomization.yml" || f == "Kustomization"
}

func kustomizationFilename(k string) (string, error) {
	if f, err := os.Stat(k); err != nil {
		// Regardless of what got passed in, it needs to exist
		return "", err
	} else if f.IsDir() {
		// Iterate over directory contents, take the first match (let Kustomize do the real validation)
		dir, err := ioutil.ReadDir(k)
		if err != nil {
			return "", err
		}
		for _, ff := range dir {
			if isRecognizedKustomizationFilename(ff.Name()) {
				return filepath.Join(k, ff.Name()), nil
			}
		}
	} else if isRecognizedKustomizationFilename(f.Name()) {
		// We were given a valid kustomization filename to begin with
		return k, nil
	}
	return "", fmt.Errorf("invalid kustomization: %s", k)
}
