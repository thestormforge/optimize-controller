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
	cmdutil "github.com/redskyops/redskyops-controller/pkg/redskyctl/util"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commander"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/config"
	"github.com/redskyops/redskyops-ui/ui"
	"github.com/spf13/cobra"
)

type Options struct {
	// Config is the Red Sky Configuration to proxy
	Config config.Config
	// IOStreams are used to access the standard process streams
	commander.IOStreams

	ServerAddress string
	DisplayURL    bool
}

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

	cmd.Flags().StringVar(&o.ServerAddress, "address", "", "Address to listen on.")
	cmd.Flags().BoolVar(&o.DisplayURL, "url", false, "Display the URL instead of opening a browser.")

	commander.ExitOnError(cmd)
	return cmd
}

func (o *Options) Complete() {
	if o.ServerAddress == "" {
		o.ServerAddress = ":0"
	}
}

func (o *Options) results(cliCtx context.Context) error {
	// Create a context we can use to shutdown the server
	ctx, shutdown := context.WithCancel(cliCtx)

	// Create the router to match requests
	router := http.NewServeMux()
	if err := o.handleAPI(router, "/v1/"); err != nil {
		shutdown() // TODO Just defer shutdown instead? Should be a no-op in the success case...
		return err
	}
	o.handleUI(router, "/ui/")
	o.handleLiveness(router, "/health")
	o.handleShutdown(router, "/shutdown", shutdown)

	// Create the server
	server := cmdutil.NewContextServer(ctx, router,
		cmdutil.WithServerOptions(o.configureServer),
		cmdutil.ShutdownOnInterrupt(func() { _, _ = fmt.Fprintln(o.Out) }),
		cmdutil.HandleStart(o.openBrowser))

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
	return browser.OpenURL(loc)
}

func (o *Options) handleAPI(serveMux *http.ServeMux, prefix string) error {
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

	// Configure a transport to provide OAuth2 tokens to the backend
	// TODO Set a UA round-tripper with both redskyctl and "rewrite proxy" as products?
	transport, err := o.Config.Authorize(context.Background(), nil)
	if err != nil {
		return err
	}

	// TODO Modify the response to include redskyctl in the Server header?
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

func (o *Options) handleShutdown(serveMux *http.ServeMux, prefix string, shutdown context.CancelFunc) {
	serveMux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
		// Invoking shutdown will bring down the server
		shutdown()

		// Print an extra newline to make up for the one we didn't print earlier
		_, _ = fmt.Fprintln(o.Out)
	})
}
