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
	// Update forwarding headers
	ApplyForwardedToOutgoingRequest(request, "_redskyctl")

	// Overwrite the request address
	request.Host = ""
	request.URL.Scheme = p.Address.Scheme
	request.URL.Host = p.Address.Host
	request.URL.Path = p.Address.Path + strings.TrimLeft(request.URL.Path, "/") // path.Join eats trailing slashes
	// TODO This requires that Address.Path ends with "/", where can we enforce that?
}

func (p *RewriteProxy) Incoming(response *http.Response) error {
	if err := rewriteLocation(response, &p.Address); err != nil {
		return err
	}
	if err := rewriteBody(response, &p.Address); err != nil {
		return err
	}
	return nil
}

func rewriteLocation(response *http.Response, address *url.URL) error {
	loc, err := url.Parse(response.Header.Get("Location"))
	if err == nil && loc.Scheme == address.Scheme && loc.Host == address.Host {
		applyOriginURL(response.Request, loc)
		response.Header.Set("Location", loc.String())
	}
	return nil
}

func rewriteBody(response *http.Response, address *url.URL) error {
	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if err := response.Body.Close(); err != nil {
		return err
	}

	loc := *address
	applyOriginURL(response.Request, &loc)

	old := []byte(address.String())
	new := []byte(loc.String())
	buf := bytes.NewBuffer(bytes.ReplaceAll(b, old, new))
	response.Body = ioutil.NopCloser(buf)
	response.ContentLength = int64(buf.Len())
	response.Header.Set("Content-Length", strconv.Itoa(buf.Len()))
	return nil
}

func applyOriginURL(request *http.Request, loc *url.URL) {
	// We don't know about the proxies in front of us, if they do not include
	// forwarding information the best we can do is assume they are running right next to us
	fwd := ParseForwarded(request.Header)

	for i := range fwd {
		if fwd[i].Proto != "" {
			loc.Scheme = fwd[i].Proto
			break
		}
	}

	for i := range fwd {
		if fwd[i].Host != "" {
			loc.Host = fwd[i].Host
			break
		}
	}
}
