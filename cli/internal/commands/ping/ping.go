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

package ping

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/internal/version"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"github.com/thestormforge/optimize-go/pkg/config"
	"golang.org/x/oauth2"
)

type Options struct {
	// Config is the Optimize Configuration
	Config *config.OptimizeConfig
	// ExperimentsAPI is used to interact with the Optimize Experiments API
	ExperimentsAPI experimentsv1alpha1.API
	// IOStreams are used to access the standard process streams
	commander.IOStreams
}

// NewPingCommand creates a new command for pinging the Optimize API
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ping",
		Short: "Ping the StormForge Optimize API",

		PreRunE: func(cmd *cobra.Command, args []string) error {
			commander.SetStreams(&o.IOStreams, cmd)
			return commander.SetExperimentsAPI(&o.ExperimentsAPI, o.Config, cmd)
		},
		RunE: commander.WithContextE(o.ping),
	}

	return cmd
}

func (o *Options) ping(ctx context.Context) error {
	r := o.Config.Reader()
	host, addrs, err := hostAndAddrs(ctx, r)
	if err != nil {
		return err
	}

	updateUserAgent(ctx)

	_, _ = fmt.Fprintf(o.Out, "PING %s (%s): HTTP/1.1 OPTIONS\n", host, strings.Join(addrs, ", "))

	start := time.Now()
	_, err = o.ExperimentsAPI.Options(ctx)
	dur := time.Since(start).Round(time.Microsecond)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(o.Out, "PONG time=%s\n", dur.String())
	return nil
}

// Returns the host name and resolved addresses of the experiments API.
func hostAndAddrs(ctx context.Context, r config.Reader) (string, []string, error) {
	srv, err := config.CurrentServer(r)
	if err != nil {
		return "", nil, err
	}

	u, err := url.Parse(srv.API.ExperimentsEndpoint)
	if err != nil {
		return "", nil, err
	}

	host := u.Hostname()
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return "", nil, err
	}
	return host, addrs, nil
}

// Adds a comment to the UA string so we know the source of all these OPTIONS requests
func updateUserAgent(ctx context.Context) {
	if rt, ok := oauth2.NewClient(ctx, nil).Transport.(*version.Transport); ok {
		rt.UserAgent += " (ping)"
	}
}
