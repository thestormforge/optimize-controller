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

package check

import (
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

const (
	checkLong    = `Run a consistency check on Red Sky Ops components`
	checkExample = ``
)

func NewCheckCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "check",
		Short:   "Run a consistency check",
		Long:    checkLong,
		Example: checkExample,
	}
	cmd.Run = cmd.HelpFunc()

	cmd.AddCommand(NewCheckExperimentCommand(f, ioStreams))
	cmd.AddCommand(NewServerCheckCommand(f, ioStreams))

	// TODO Add a manager check?

	return cmd
}
