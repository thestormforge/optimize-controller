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

package completion

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Options is the configuration for creation shell completion scripts
type Options struct {
	Shell string
}

func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion SHELL",
		Short: "Output shell completion code",
		Long:  "Output shell completion code which can be evaluated to provide interactive completion of commands.",

		Example: `# Load the completion code for zsh into the current shell
source <(stormforge completion zsh)
# Set the completion code for zsh to autoload (assuming '$ZSH/completions' is part of 'fpath')
stormforge completion zsh > $ZSH/completions/_stormforge`,

		Args:      cobra.ExactValidArgs(1),
		ValidArgs: []string{"bash", "fish", "zsh"},

		PreRun: func(_ *cobra.Command, args []string) { o.Shell = args[0] },
		RunE:   func(cmd *cobra.Command, _ []string) error { return o.completion(cmd) },
	}

	return cmd
}

func (o *Options) completion(cmd *cobra.Command) error {
	switch o.Shell {
	case "bash":
		return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
	case "fish":
		return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
	case "zsh":
		return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
	default:
		return fmt.Errorf("completion is not implemented for %s", o.Shell)
	}
}
