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
package util

import (
	"os/exec"

	"github.com/spf13/cobra"
)

type Kubectl struct {
	Bin string

	// TODO Do we need to propagate other connection information to kubectl, just have a "globalArgs" array?
	// TODO We should probably support at least a --context option...
}

func NewKubectl() *Kubectl {
	return &Kubectl{}
}

func (k *Kubectl) AddFlags(cmd *cobra.Command) {}

func (k *Kubectl) Complete() error {
	if k.Bin == "" {
		k.Bin = "kubectl"
	}
	return nil
}

func (k *Kubectl) NewCmd(args ...string) *exec.Cmd {
	return exec.Command(k.Bin, args...)
}
