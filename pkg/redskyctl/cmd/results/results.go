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
	"net/url"
	"strings"

	"github.com/redskyops/k8s-experiment/pkg/api"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/redskyops/redskyops-ui/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	resultsLong    = ``
	resultsExample = ``
)

// TODO Should we fork a browser process with the correct URL?
// TODO Should we default to something different for ServerAddress?

type ResultsOptions struct {
	ServerAddress string
	BackendConfig *viper.Viper

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

	return cmd
}

func (o *ResultsOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	if o.BackendConfig == nil {
		var err error
		if o.BackendConfig, err = f.ToClientConfig(); err != nil {
			return err
		}
	}
	return nil
}

func (o *ResultsOptions) Run() error {
	apiHandler, err := o.apiHandler()
	if err != nil {
		return err
	}

	uiHandler, err := o.uiHandler("/ui/")
	if err != nil {
		return err
	}

	http.Handle("/api/", apiHandler)
	http.Handle("/ui/", uiHandler)
	return http.ListenAndServe(o.ServerAddress, nil)
}

func (o *ResultsOptions) apiHandler() (http.Handler, error) {
	// Configure a director to rewrite request URLs
	address, err := api.GetAddress(o.BackendConfig)
	if err != nil {
		return nil, err
	}
	director := direct(address)

	// Configure a transport to provide OAuth2 tokens to the backend
	transport, err := api.ConfigureOAuth2(o.BackendConfig, context.Background(), nil)
	if err != nil {
		return nil, err
	}

	return &httputil.ReverseProxy{Director: director, Transport: transport}, nil
}

func (o *ResultsOptions) uiHandler(prefix string) (http.Handler, error) {
	return http.StripPrefix(prefix, http.FileServer(ui.Assets)), nil
}

// Returns a reverse proxy director that rewrite the request URL to point to the API at the configured address
func direct(address *url.URL) func(r *http.Request) {
	return func(request *http.Request) {
		// Update forwarding headers
		request.Header.Set("Forwarded", fmt.Sprintf("proto=http;host=%s", request.Host))
		request.Header.Set("X-Forwarded-Proto", "http")
		request.Header.Set("X-Forwarded-Host", request.Host)
		request.Host = ""

		// Overwrite the request address
		request.URL.Scheme = address.Scheme
		request.URL.Host = address.Host
		request.URL.Path = address.Path + strings.TrimLeft(request.URL.Path, "/") // path.Join eats trailing slashes

		// TODO Should we limit this to only the API required by the UI?
	}
}
