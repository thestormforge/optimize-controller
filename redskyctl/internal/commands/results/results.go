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
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os/user"
	"strings"
	"time"

	"github.com/pkg/browser"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-ui/v2/ui"
	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-go/pkg/redskyapi"
	"golang.org/x/oauth2"
)

// Options is the configuration for displaying the results UI
type Options struct {
	// Config is the Red Sky Configuration to proxy
	Config redskyapi.Config
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	// ServerAddress is the address to listen on (defaults to an ephemeral port)
	ServerAddress string
	// DisplayURL just prints the URL instead of opening the default browser
	DisplayURL bool
	// IdleTimeout is the time between heartbeats to the "/health" endpoint required to keep the server up (defaults to 5 seconds)
	IdleTimeout time.Duration
}

// NewCommand creates a new command for displaying the results UI
func NewCommand(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "results",
		Short: "Serve a visualization of the results",

		PreRun: func(cmd *cobra.Command, args []string) {
			commander.SetStreams(&o.IOStreams, cmd)
			o.Complete()
		},
		RunE: commander.WithContextE(o.results),
	}

	cmd.Flags().StringVar(&o.ServerAddress, "address", "", "address to listen on")
	cmd.Flags().BoolVar(&o.DisplayURL, "url", false, "display the URL instead of opening a browser")
	cmd.Flags().DurationVar(&o.IdleTimeout, "idle-timeout", 5*time.Second, "set the heartbeat interval (0 to ignore heartbeats)")
	_ = cmd.Flags().MarkHidden("idle-timeout")

	return cmd
}

func (o *Options) Complete() {
	// Default to listening on an ephemeral port
	if o.ServerAddress == "" {
		o.ServerAddress = ":0"
	}

	// If we are just printing a URL we can't rely on the heartbeat to keep the process alive
	if o.DisplayURL {
		o.IdleTimeout = 0
	}
}

func (o *Options) results(ctx context.Context) error {
	// Create the router to match requests
	router := http.NewServeMux()
	if err := o.handleAPI(ctx, router, "/v1/"); err != nil {
		return err
	}
	o.handleUI(router, "/ui/")
	o.handleLiveness(router, "/health")

	// Create the server
	server := commander.NewContextServer(ctx, router,
		commander.WithServerOptions(o.configureServer),
		commander.ShutdownOnInterrupt(func() { _, _ = fmt.Fprintln(o.Out) }),
		commander.ShutdownOnIdle(o.IdleTimeout, func() { _, _ = fmt.Fprintln(o.Out) }),
		commander.HandleStart(o.openBrowser))

	// Start the server, this will block until someone calls 'shutdown' from above
	return server.ListenAndServe()
}

func (o *Options) configureServer(srv *http.Server) {
	srv.Addr = o.ServerAddress
	srv.ReadTimeout = 5 * time.Second
	srv.WriteTimeout = 10 * time.Second
	srv.IdleTimeout = 15 * time.Second
}

func (o *Options) openBrowser(loc string) error {
	u, err := user.Current()
	if err != nil {
		return err
	}

	// Do not open the browser for root
	if o.DisplayURL || u.Uid == "0" {
		_, _ = fmt.Fprintf(o.Out, "%s\n", loc)
		return nil
	}

	_, _ = fmt.Fprintf(o.Out, "Opening %s in your default browser...", loc)
	if err := browser.OpenURL(loc); err != nil {
		return fmt.Errorf("failed to open browser, use 'redskyctl results --url' instead")
	}
	return nil
}

func (o *Options) handleAPI(ctx context.Context, serveMux *http.ServeMux, prefix string) error {
	// Get the endpoint mappings from the configuration
	// TODO Should we just store the endpoints mapping on the rewrite proxy?
	endpoints, err := o.Config.Endpoints()
	if err != nil {
		return err
	}

	// Assume the experiments endpoint can be used to get a base path into the remote server
	address := endpoints.Resolve("/experiments/")
	address.Path = strings.TrimSuffix(address.Path, "/experiments/")
	rp := &RewriteProxy{Address: *address}

	// TODO We need bi-directional middleware for the client and server to set UA and Server headers
	// We should know the UI version when we set the UA string so both products are present
	// This is a temporary workaround: we can steal the UA transport from the default OAuth2 client
	rt := oauth2.NewClient(ctx, nil).Transport

	// Configure a transport to provide OAuth2 tokens to the backend
	transport, err := o.Config.Authorize(ctx, rt)
	if err != nil {
		return err
	}

	serveMux.Handle(prefix, http.StripPrefix(prefix, &httputil.ReverseProxy{
		Director:       rp.Outgoing,
		ModifyResponse: rp.Incoming,
		Transport:      transport,
	}))
	return nil
}

func (o *Options) handleUI(serveMux *http.ServeMux, prefix string) {
	serveMux.Handle("/", http.RedirectHandler(prefix, http.StatusMovedPermanently))
	serveMux.Handle(prefix, http.StripPrefix(prefix, http.FileServer(ui.Assets)))
}

func (o *Options) handleLiveness(serveMux *http.ServeMux, prefix string) {
	serveMux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.Error(w, "ok", http.StatusOK)
	})
}
