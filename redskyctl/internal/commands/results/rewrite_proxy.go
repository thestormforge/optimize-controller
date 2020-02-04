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
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type RewriteProxy struct {
	Address url.URL
}

func (p *RewriteProxy) Outgoing(request *http.Request) {
	// Ensure the forwarding headers are set
	if request.Header.Get("X-Forwarded-Host") == "" {
		request.Header.Set("X-Forwarded-Host", request.Host) // TODO Can this still be empty?
	}
	if request.Header.Get("X-Forwarded-Proto") == "" {
		forwardedProto := "http"
		if request.TLS != nil { // TODO Is this right?
			forwardedProto = "https"
		}
		request.Header.Set("X-Forwarded-Proto", forwardedProto)
	}

	// Set the server side established host value to empty so the client side will use the URL for the `Host` header
	request.Host = ""

	// Overwrite the request address; do not bother with merging the query strings
	request.URL.Scheme = p.Address.Scheme
	request.URL.Host = p.Address.Host
	request.URL.Path = strings.TrimRight(p.Address.Path, "/") + "/" + strings.TrimLeft(request.URL.Path, "/")

	// NewSingleHostReverseProxy does this, so it must be right
	if _, ok := request.Header["User-Agent"]; !ok {
		request.Header.Set("User-Agent", "")
	}
}

func (p *RewriteProxy) Incoming(response *http.Response) error {
	loc, err := url.Parse(response.Header.Get("Location"))
	if err != nil {
		return err
	}

	if loc.Scheme == p.Address.Scheme && loc.Host == p.Address.Host {
		response.Header.Set("Location", forwarded(loc, response))
	}

	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if err := response.Body.Close(); err != nil {
		return err
	}

	buf := bytes.NewBuffer(bytes.ReplaceAll(b, []byte(p.Address.String()), []byte(forwarded(&p.Address, response))))
	response.Body = ioutil.NopCloser(buf)
	response.ContentLength = int64(buf.Len())
	response.Header.Set("Content-Length", strconv.Itoa(buf.Len()))

	return nil
}

// Since we always set the X-Forwarded-* headers before sending the request, we can use them to fix the response
func forwarded(u *url.URL, r *http.Response) string {
	loc := *u
	loc.Scheme = r.Request.Header.Get("X-Forwarded-Proto")
	loc.Host = r.Request.Header.Get("X-Forwarded-Host")
	return loc.String()
}
