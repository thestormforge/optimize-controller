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

package setup

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"
)

// NOTE: The `authenticationProvider` is throw-away code! It should be replaced by a real, external provider

// NOTE: None of the access to the dummy AP is synchronized (unless someone is poking around, it shouldn't matter)

// loginFormPage is a simple login form with three inputs (the client ID & secret, plus the redirect URL)
const loginFormPage = `<!DOCTYPE html>
<html>
<head>
	<title>Sign In</title>
	<style>*{font-family:Montserrat,sans-serif;font-size:14px}html{height:100%}body{background:radial-gradient(#40404b,#111118) rgba(34,34,40,.94);padding:40px}form{width:360px;margin:auto;background:#fff;border-radius:3px;padding:40px}form img{display:block;margin:auto;width:50%}form p{margin-top:10px;text-align:center;color:#f55467;font-size:26px}form input{width:340px;padding:9px;margin:2px;border:1px solid #d1cdcc;border-radius:3px}form button{width:100%;padding:10px;background:#f55467;color:#f9f9f9;font-weight:600;border:0;border-radius:3px;margin:2px;margin-top:20px}</style>
</head>
<body>
<form action="/login" method="POST">
<div class="logo"><img src="https://redskyops.dev/img/redskylogo.png"><p>Red Sky Ops</p></div>
<input type="text" name="client_id" id="client_id" placeholder="Client ID" autocomplete="off" autocapitalize="off" autofocus>
<input type="text" name="client_secret" id="client_secret" placeholder="Client secret" autocomplete="off" autocapitalize="off">
<input type="hidden" name="redirect_uri" value="{{.RedirectURL}}">
<button name="login" type="submit">Log In</button>
</form>
<script type="text/javascript">function pb(e){try{var t=JSON.parse((e.clipboardData||window.clipboardData).getData("Text"));if(t&&"object"==typeof t&&(t.client_id||t.client_secret))return e.stopPropagation(),e.preventDefault(),t.client_id&&(document.getElementById("client_id").value=t.client_id),void(t.client_secret&&(document.getElementById("client_secret").value=t.client_secret))}catch(e){}}document.getElementById("client_id").addEventListener("paste",pb),document.getElementById("client_secret").addEventListener("paste",pb);</script>
</body>
</html>
`

// authenticationProvider is a provider which we can use to make a two-legged flow look like it has a third leg.
type authenticationProvider struct {
	// Salt is regenerated internally for each login request
	Salt []byte
	// ClientID is supplied by the user for each login request
	ClientID string
	// ClientSecret is supplied by the user for each login request
	ClientSecret string
	// state is supplied by the client to mitigate CSRF attacks
	state string
}

// register this authentication provider with the supplied mux at the specified base URL and get the OAuth endpoint back
func (ap *authenticationProvider) register(baseURL *url.URL, router *http.ServeMux) *oauth2.Endpoint {
	authURL := *baseURL
	authURL.Path = "/authorize"
	tokenURL := *baseURL
	tokenURL.Path = "/oauth/token"

	router.HandleFunc("/login", ap.login)
	router.HandleFunc(authURL.Path, ap.authorize)
	router.HandleFunc(tokenURL.Path, ap.token)

	return &oauth2.Endpoint{
		AuthURL:   authURL.String(),
		TokenURL:  tokenURL.String(),
		AuthStyle: oauth2.AuthStyleInParams,
	}
}

// returns a hash of the current state; used to ensure the login request matches the token exchange
func (ap *authenticationProvider) code() string {
	h := sha256.New()
	_, _ = h.Write(ap.Salt)
	_, _ = h.Write([]byte(ap.ClientID))
	_, _ = h.Write([]byte(ap.ClientSecret))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// authorize handles the initial request for a login
func (ap *authenticationProvider) authorize(w http.ResponseWriter, r *http.Request) {
	// Verify this is a GET method
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Store request parameters for later
	ap.state = r.FormValue("state")
	// TODO We should also store the code challenge so we can verify it later

	// We should only need this once so don't bother caching the parsed templates
	tmpl, err := template.New(",loginForm").Parse(loginFormPage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p := make(map[string]interface{})
	p["RedirectURL"] = r.FormValue("redirect_uri")
	// TODO Error message from previous failed login attempt?

	err = tmpl.Execute(w, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// login handles the POST request from the dummy login page
func (ap *authenticationProvider) login(w http.ResponseWriter, r *http.Request) {
	// Verify this is a POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Overwrite the salt
	ap.Salt = make([]byte, 8)
	_, err := rand.Read(ap.Salt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse the redirect URL
	redirectURL, err := url.Parse(r.PostFormValue("redirect_uri"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get the form fields
	ap.ClientID = r.PostFormValue("client_id")
	ap.ClientSecret = r.PostFormValue("client_secret")

	// TODO How do we get the address?
	// TODO Make a mock request to the API to see if it works
	// TODO On failed auth, we need to redirect back to the login page with an error message...

	// Add the code to the redirect URL
	query := redirectURL.Query()
	query.Set("code", ap.code())
	query.Set("state", ap.state)
	redirectURL.RawQuery = query.Encode()

	// Redirect back to the client (who will exchange the authorization code for an access token)
	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

// token generates an actual token
func (ap *authenticationProvider) token(w http.ResponseWriter, r *http.Request) {
	// Verify this is a POST request
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO We should verify the code challenge here

	// Verify the request matches the current state
	if r.FormValue("code") != ap.code() {
		http.Error(w, "Failed to validate request", http.StatusUnauthorized)
		return
	}

	// Generate an access token that we use to transport the client credentials
	accessToken := make(map[string]string)
	accessToken["redsky_client_id"] = ap.ClientID
	accessToken["redsky_client_secret"] = ap.ClientSecret
	b, err := json.Marshal(accessToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate a dummy OAuth2 token
	token := url.Values{}
	token.Set("access_token", base64.URLEncoding.EncodeToString(b))
	token.Set("token_type", "dummy")
	token.Set("refresh_token", "impossible")
	token.Set("expires_in", "1")
	_, _ = w.Write([]byte(token.Encode()))
}
