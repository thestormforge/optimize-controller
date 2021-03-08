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

package scan

import (
	"io"
	"os/exec"

	"github.com/thestormforge/konjure/pkg/konjure"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
)

// NewKonjureFilter creates a new Konjure filter with the default settings
// for this project. The supplied default reader is used when the "-" is requested.
func NewKonjureFilter(defaultReader io.Reader) *konjure.Filter {
	return &konjure.Filter{
		Depth:         100,
		DefaultReader: defaultReader,
		KeepStatus:    true,

		// Override the default behaviors for execution
		KubectlExecutor:   kubectl,
		KustomizeExecutor: kustomize,
	}
}

func kubectl(cmd *exec.Cmd) ([]byte, error) {
	// TODO We can use the Kube API to handle `kubectl get`, we may need pflags to help
	return cmd.Output()
}

func kustomize(cmd *exec.Cmd) ([]byte, error) {
	// If the command path is absolute, it was found by LookPath. In this
	// case we will just fork so the embedded version of Kustomize does not
	// becoming a limiting factor.

	// We are only handling the very specific case of `kustomize build X`
	if len(cmd.Args) != 3 || cmd.Path != "kustomize" || cmd.Args[1] != "build" {
		return cmd.Output()
	}

	// Restrict to disk access to be consistent with the flow when we fork
	fs := filesys.MakeFsOnDisk()

	// Create Krusty options: be sure to disable KYAML as we don't want to accidentally
	// invoke code paths in which we have created problems due to dependencies
	opts := &krusty.Options{
		UseKyaml:         false,
		LoadRestrictions: types.LoadRestrictionsNone,
		PluginConfig:     konfig.DisabledPluginConfig(),
	}

	// Run the Kustomization in process
	rm, err := krusty.MakeKustomizer(fs, opts).Run(cmd.Args[1])
	if err != nil {
		return nil, err
	}

	return rm.AsYaml()
}
