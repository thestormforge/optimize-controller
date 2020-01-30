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

package version

import (
	"net/http"
	"strings"
)

// UserAgent wraps the (possibly nil) transport so that it will set the user agent
// using the supplied product name and current version
func UserAgent(product string, transport http.RoundTripper) http.RoundTripper {
	return &Transport{
		UserAgent: userAgentString(product),
		Base:      transport,
	}
}

// Transport sets the `User-Agent` header
type Transport struct {
	// UserAgent string to use, defaults to "RedSky/{version}" if unset
	UserAgent string
	// Base transport to use, uses the system default if nil
	Base http.RoundTripper
}

// RoundTrip sets the incoming request header and delegates to the base transport
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", t.userAgent())
	return t.base().RoundTrip(req)
}

func (t *Transport) userAgent() string {
	if t.UserAgent != "" {
		return t.UserAgent
	}
	// TODO We probably don't want to be doing this every time...
	return userAgentString("RedSky")
}

func (t *Transport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

func userAgentString(product string) string {
	if product == "" {
		return ""
	}
	// TODO Validate "product"
	return product + "/" + strings.TrimLeft(Version, "v")
}
