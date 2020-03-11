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
	"github.com/spf13/cobra"
)

// NewCommand returns a new Kustomization integration command
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kustomize",
		Short: "Kustomize integrations",
		Long:  "Kustomize integrations for Red Sky Ops",
	}

	cmd.AddCommand(NewConfigCommand(&ConfigOptions{}))

	return cmd
}
