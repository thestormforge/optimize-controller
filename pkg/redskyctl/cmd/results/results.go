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
	"time"

	"github.com/pkg/browser"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	redskyclient "github.com/redskyops/k8s-experiment/redskyapi"
	"github.com/redskyops/redskyops-ui/ui"
	"github.com/spf13/cobra"
)

const (
	resultsLong    = ``
	resultsExample = ``
)

type ResultsOptions struct {
	ServerAddress string
	DisplayURL    bool
	BackendConfig redskyclient.Config

	cmdutil.IOStreams
}

func NewResultsOptions(ioStreams cmdutil.IOStreams) *ResultsOptions {
	return &ResultsOptions{
		IOStreams: ioStreams,
	}
}

func NewResultsCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewResultsOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "results",
		Short:   "Serve a visualization of the results",
		Long:    resultsLong,
		Example: resultsExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.ServerAddress, "address", "", "Address to listen on.")
	cmd.Flags().BoolVar(&o.DisplayURL, "url", false, "Display the URL instead of opening a browser.")

	return cmd
}

func (o *ResultsOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	if o.ServerAddress == "" {
		o.ServerAddress = ":0"
	}

	if o.BackendConfig == nil {
		var err error
		if o.BackendConfig, err = f.ToClientConfig(); err != nil {
			return err
		}
	}
	return nil
}

func (o *ResultsOptions) Run() error {
	// Create the router to match requests
	router := http.NewServeMux()
	if err := o.handleUI(router, "/ui/"); err != nil {
		return err
	}
	if err := o.handleAPI(router, "/api/"); err != nil {
		return err
	}
	if err := o.handleLiveness(router, "/health"); err != nil {
		return err
	}
	ctx, err := o.handleShutdown(router, "/shutdown")
	if err != nil {
		return err
	}

	// Create the server
	server := cmdutil.NewContextServer(ctx, router,
		cmdutil.WithServerOptions(o.configureServer),
		cmdutil.ShutdownOnInterrupt(func() { _, _ = fmt.Fprintln(o.Out) }),
		cmdutil.HandleStart(o.openBrowser))

	return server.ListenAndServe()
}

func (o *ResultsOptions) configureServer(srv *http.Server) {
	srv.Addr = o.ServerAddress
	srv.ReadTimeout = 5 * time.Second
	srv.WriteTimeout = 10 * time.Second
	srv.IdleTimeout = 15 * time.Second
}

func (o *ResultsOptions) openBrowser(loc string) error {
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

func (o *ResultsOptions) handleUI(serveMux *http.ServeMux, prefix string) error {
	serveMux.Handle("/", http.RedirectHandler(prefix, http.StatusMovedPermanently))
	serveMux.Handle(prefix, http.StripPrefix(prefix, http.FileServer(ui.Assets)))
	return nil
}

func (o *ResultsOptions) handleAPI(serveMux *http.ServeMux, prefix string) error {
	// Configure a director to rewrite request URLs
	// TODO This is probably going to double-up "/experiments/experiments/"
	// The backend doesn't really support context roots so we may need to either change the UI to use "/v1"
	// instead of "/api" (and change our proxy); or get more aggressive with our rewriting to keep "/api"
	endpoints, err := o.BackendConfig.Endpoints()
	if err != nil {
		return err
	}
	address := endpoints.Resolve("/experiments/")
	rp := &RewriteProxy{Address: *address}

	// Configure a transport to provide OAuth2 tokens to the backend
	// TODO Set a UA round-tripper with both redskyctl and "rewrite proxy" as products?
	transport, err := o.BackendConfig.Authorize(context.Background(), nil)
	if err != nil {
		return err
	}

	// TODO Modify the response to include redskyctl in the Server header?
	serveMux.Handle(prefix, &httputil.ReverseProxy{
		Director:       rp.Outgoing,
		ModifyResponse: rp.Incoming,
		Transport:      transport,
	})
	return nil
}

func (o *ResultsOptions) handleLiveness(serveMux *http.ServeMux, prefix string) error {
	serveMux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fmt.Fprint(w, "ok")
	})
	return nil
}

func (o *ResultsOptions) handleShutdown(serveMux *http.ServeMux, prefix string) (context.Context, error) {
	ctx, shutdown := context.WithCancel(context.Background())
	serveMux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
		shutdown()
		_, _ = fmt.Fprintln(o.Out)
	})
	return ctx, nil
}
