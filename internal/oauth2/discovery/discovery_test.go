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

package discovery

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestWellKnownURI(t *testing.T) {
	g := NewWithT(t)

	g.Expect(WellKnownURI("http://example.com", "")).To(Equal("http://example.com/.well-known/"))
	g.Expect(WellKnownURI("http://example.com", "foo")).To(Equal("http://example.com/.well-known/foo"))
	g.Expect(WellKnownURI("http://example.com/", "foo")).To(Equal("http://example.com/.well-known/foo"))
	g.Expect(WellKnownURI("http://example.com/x", "foo")).To(Equal("http://example.com/.well-known/foo/x"))

	g.Expect(WellKnownURI("", "")).To(Equal("/.well-known/"))
	g.Expect(WellKnownURI("", "foo")).To(Equal("/.well-known/foo"))
	g.Expect(WellKnownURI("/", "foo")).To(Equal("/.well-known/foo"))
	g.Expect(WellKnownURI("/x", "foo")).To(Equal("/.well-known/foo/x"))
}
