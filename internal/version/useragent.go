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
func UserAgent(product, comment string, transport http.RoundTripper) http.RoundTripper {
	return &Transport{
		UserAgent: userAgentString(product, comment),
		Base:      transport,
	}
}

// Transport sets the `User-Agent` header
type Transport struct {
	// UserAgent string to use, defaults to "Optimize/{version}" if unset
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
	return userAgentString("Optimize", "")
}

func (t *Transport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

func userAgentString(product, comment string) string {
	if product == "" {
		return ""
	}

	// TODO Validate "product"
	ua := strings.Builder{}
	ua.WriteString(product)
	ua.WriteRune('/')
	ua.WriteString(strings.TrimPrefix(Version, "v"))

	// Build the comments using both the user supplied comments and some additional build information
	var comments []string

	// Only include build metadata for pre-release versions
	if strings.Contains(Version, "-") && BuildMetadata != "" {
		comments = append(comments, BuildMetadata)
	}

	// Clean white space and surrounding comment indicators
	comment = strings.TrimSpace(comment)
	comment = strings.TrimLeft(comment, "(")
	comment = strings.TrimRight(comment, ")")
	comment = strings.TrimSpace(comment)
	if comment != "" {
		comments = append(comments, comment)
	}

	// Only add the comment if it is not empty
	if len(comments) > 0 {
		ua.WriteString(" (" + strings.Join(comments, "; ") + ")")
	}

	return ua.String()
}
