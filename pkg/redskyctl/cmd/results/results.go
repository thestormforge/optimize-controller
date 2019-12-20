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
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"syscall"
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
	BackendConfig *redskyclient.Config

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
		o.ServerAddress = ":8080" // TODO Use ":0" once we figure out the listener stuff
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

	// Create the server
	server := &http.Server{
		Addr:         o.ServerAddress,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	serve, shutdown := context.WithCancel(context.Background())
	done := make(chan error, 1)

	// Start the server and a blocked shutdown routine
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			done <- err
		}
	}()
	go func() {
		<-serve.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- server.Shutdown(ctx)
	}()

	// Add a signal handler so we shutdown cleanly on SIGINT/TERM
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit
		_, _ = fmt.Fprintln(o.Out)
		shutdown()
	}()

	// Try to connect to see if start up failed
	// TODO Do we need to retry this?
	conn, err := net.DialTimeout("tcp", o.ServerAddress, 2*time.Second)
	if err == nil {
		_ = conn.Close()
	}

	// Before opening the browser, check to see if there were any errors
	select {
	case err := <-done:
		return err
	default:
		if err := o.openBrowser(); err != nil {
			shutdown()
			return err
		}
	}
	return <-done
}

func (o *ResultsOptions) url() *url.URL {
	loc := &url.URL{Scheme: "http", Host: o.ServerAddress}
	if loc.Hostname() == "" {
		loc.Host = "localhost" + loc.Host
	}
	return loc
}

func (o *ResultsOptions) openBrowser() error {
	loc := o.url().String()
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
	address, err := redskyclient.GetAddress(o.BackendConfig)
	if err != nil {
		return err
	}
	rp := &RewriteProxy{Address: *address}

	// Configure a transport to provide OAuth2 tokens to the backend
	transport, err := redskyclient.ConfigureOAuth2(o.BackendConfig, context.Background(), nil)
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
