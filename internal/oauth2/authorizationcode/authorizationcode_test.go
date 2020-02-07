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
	"net/url"
	"testing"

	. "github.com/onsi/gomega"
)

func TestConfig_AuthCodeURLWithPKCE(t *testing.T) {
	g := NewWithT(t)
	c := Config{}

	// https://tools.ietf.org/html/rfc7636#appendix-B
	vb := []byte{116, 24, 223, 180, 151, 153, 224, 37, 79, 250, 96, 125, 216, 173,
		187, 186, 22, 212, 37, 77, 105, 214, 191, 240, 91, 88, 5, 88, 83,
		132, 141, 121}

	// Make sure we used the correct encoding for the example
	c.setVerifier(vb)
	g.Expect(c.verifier).To(Equal("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"), "Example verifier")

	// Even though we did not provide an Endpoint.AuthURL, this should still produce a parsable URL
	u, err := url.Parse(c.AuthCodeURLWithPKCE())
	g.Expect(err).ShouldNot(HaveOccurred())

	// Make sure we get the expected code challenge
	v := u.Query()
	g.Expect(v.Get("code_challenge_method")).To(Equal("S256"), "Code Challenge Method")
	g.Expect(v.Get("code_challenge")).To(Equal("E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"), "Code Challenge")
}
