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

package configure

import (
	"fmt"

	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-go/pkg/config"
	"github.com/spf13/cobra"
)

// CurrentContextOptions are the options for viewing the current context
type CurrentContextOptions struct {
	// Config is the Red Sky Configuration to view
	Config *config.RedSkyConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams
}

// NewCurrentContextCommand creates a new command for viewing the current context
func NewCurrentContextCommand(o *CurrentContextOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "current-context",
		Short: "Displays the current context",
		Long:  "Displays the current context",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithoutArgsE(o.currentContext),
	}

	commander.ExitOnError(cmd)
	return cmd
}

func (o *CurrentContextOptions) currentContext() error {
	_, _ = fmt.Fprintln(o.Out, o.Config.Reader().ContextName())
	return nil
}
