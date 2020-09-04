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

package authorizationcode

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthorization(t *testing.T) {
	testCases := []struct {
		desc                string
		vb                  []byte
		verifierb64         string
		verifierText        string
		codeChallengeMethod string
		codeChallenge       string
	}{
		{
			desc: "Example for the S256 code_challenge_method",
			vb: []byte{116, 24, 223, 180, 151, 153, 224, 37, 79, 250, 96, 125, 216, 173,
				187, 186, 22, 212, 37, 77, 105, 214, 191, 240, 91, 88, 5, 88, 83,
				132, 141, 121},
			verifierb64:         "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk",
			codeChallengeMethod: "S256",
			codeChallenge:       "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			var err error
			assert.NoError(t, err)

			c, err := NewAuthorizationCodeFlowWithPKCE()
			assert.NoError(t, err)

			// Override verifier
			c.setVerifier(tc.vb)

			assert.Equal(t, tc.verifierb64, c.verifier)

			u, err := url.Parse(c.AuthCodeURLWithPKCE())
			assert.NoError(t, err)

			v := u.Query()
			assert.Equal(t, tc.codeChallengeMethod, v.Get("code_challenge_method"))
			assert.Equal(t, tc.codeChallenge, v.Get("code_challenge"))

			// Cant do exchange without a test server
			// May want to look at implementing something like
			// https://github.com/golang/oauth2/blob/master/oauth2_test.go#L140
			// _, err = c.ExchangeWithPKCE(context.Background(), "1234")
			// assert.NoError(t, err)
		})
	}
}
