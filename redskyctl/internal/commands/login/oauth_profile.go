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

package login

import (
	"strings"

	"golang.org/x/oauth2"
)

// NewOAuthConfig returns a configuration for the specified profile (or nil if the profile is not known)
func NewOAuthConfig(profile string, cfg *oauth2.Config) {
	// TODO Hack, assume the profile is actually a URL
	if strings.HasPrefix(profile, "http:") && strings.HasSuffix(profile, "/") {
		cfg.ClientID = "YOUR_CLIENT_ID_HERE"
		cfg.Endpoint.AuthURL = profile + "authorize"
		cfg.Endpoint.TokenURL = profile + "oauth/token"
		cfg.Endpoint.AuthStyle = oauth2.AuthStyleInParams
		return
	}

	switch profile {
	case "dev":
		cfg.ClientID = "TYNvHNNtl2iGKJ3k9TyECDe81vrouzOu"
		cfg.Endpoint.AuthURL = "https://redskyops-dev.auth0.com/authorize"
		cfg.Endpoint.TokenURL = "https://redskyops-dev.auth0.com/oauth/token"
		cfg.Endpoint.AuthStyle = oauth2.AuthStyleInParams
		return
	}
}
