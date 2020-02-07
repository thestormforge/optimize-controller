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

// Package discovery implements the OAuth 2.0 Authorization Server Metadata.
//
// See https://tools.ietf.org/html/rfc8414
package discovery

import "net/url"

// WellKnownURI returns a ".well-known" location for the registered name.
//
// The registry of valid "name" values can be found at https://www.iana.org/assignments/well-known-uris/well-known-uris.xhtml
func WellKnownURI(identifier, name string) string {
	u, err := url.Parse(identifier)
	if err != nil {
		// This isn't the most safe...
		return identifier + "/.well-known/" + name
	}
	if u.Path != "/" {
		name += u.Path
	}
	u.Path = "/.well-known/" + name
	return u.String()
}
