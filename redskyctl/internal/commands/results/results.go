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

package results

import (
	"fmt"
	"os/user"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/redskyctl/internal/commander"
	"github.com/thestormforge/optimize-go/pkg/config"
)

// Options is the configuration for displaying the results UI
type Options struct {
	// Config is the Optimize Configuration to get redirect URLs from
	Config *config.OptimizeConfig
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	ServerAddress string
	DisplayURL    bool
	IdleTimeout   time.Duration
}

// NewCommand creates a new command for displaying the results UI
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "results",
		Short:      "View a visualization of the results",
		Deprecated: "you can now access your results anytime using the web interface",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithoutArgsE(o.results),
	}

	// Keep the flags so we don't fail, but mark the all as hidden
	cmd.Flags().StringVar(&o.ServerAddress, "address", "", "ignored for compatibility")
	cmd.Flags().BoolVar(&o.DisplayURL, "url", false, "display the URL instead of opening a browser")
	cmd.Flags().DurationVar(&o.IdleTimeout, "idle-timeout", 5*time.Second, "ignored for compatibility")
	_ = cmd.Flags().MarkHidden("address")
	_ = cmd.Flags().MarkHidden("url")
	_ = cmd.Flags().MarkHidden("idle-timeout")

	return cmd
}

func (o *Options) results() error {
	s, err := config.CurrentServer(o.Config.Reader())
	if err != nil {
		return err
	}

	u, err := user.Current()
	if err != nil {
		return err
	}

	loc := s.Application.ExperimentsEndpoint

	// Do not open the browser for root
	if o.DisplayURL || u.Uid == "0" {
		_, _ = fmt.Fprintf(o.Out, "%s\n", loc)
		return nil
	}

	_, _ = fmt.Fprintf(o.Out, "Opening %s in your default browser...\n", loc)
	if err := browser.OpenURL(loc); err != nil {
		return fmt.Errorf("failed to open browser, use 'redskyctl results --url' instead")
	}

	return nil
}
