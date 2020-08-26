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

package config

import (
	"testing"

	. "github.com/onsi/gomega"
)

const (
	// Dummy endpoint names for testing
	exp             = "/experiments/"
	expFooBar       = "/experiments/foo_bar"
	expFooBarTrials = "/experiments/foo_bar/trials/"
)

func TestRedSkyConfig_Endpoints(t *testing.T) {
	g := NewWithT(t)

	cfg := &RedSkyConfig{}
	g.Expect(defaultLoader(cfg)).Should(Succeed())

	// This is the main use case using the default data
	rss := &cfg.data.Servers[0].Server.RedSky
	ep, err := cfg.Endpoints()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(ep.Resolve(exp).String()).To(Equal("https://api.carbonrelay.io/v1/experiments/"))
	g.Expect(ep.Resolve(expFooBar).String()).To(Equal("https://api.carbonrelay.io/v1/experiments/foo_bar"))
	g.Expect(ep.Resolve(expFooBarTrials).String()).To(Equal("https://api.carbonrelay.io/v1/experiments/foo_bar/trials/"))

	// Change inside the path
	rss.ExperimentsEndpoint = "http://example.com/api/experiments/"
	ep, err = cfg.Endpoints()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(ep.Resolve(exp).String()).To(Equal("http://example.com/api/experiments/"))
	g.Expect(ep.Resolve(expFooBar).String()).To(Equal("http://example.com/api/experiments/foo_bar"))
	g.Expect(ep.Resolve(expFooBarTrials).String()).To(Equal("http://example.com/api/experiments/foo_bar/trials/"))

	// Missing trailing slash in the configuration data
	rss.ExperimentsEndpoint = "http://example.com/api/experiments"
	ep, err = cfg.Endpoints()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(ep.Resolve(exp).String()).To(Equal("http://example.com/api/experiments/"))
	g.Expect(ep.Resolve(expFooBar).String()).To(Equal("http://example.com/api/experiments/foo_bar"))
	g.Expect(ep.Resolve(expFooBarTrials).String()).To(Equal("http://example.com/api/experiments/foo_bar/trials/"))

	// Query string in the configuration data
	rss.ExperimentsEndpoint = "http://example.com/api/experiments?foo=bar"
	ep, err = cfg.Endpoints()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(ep.Resolve(exp).String()).To(Equal("http://example.com/api/experiments/?foo=bar"))
	g.Expect(ep.Resolve(expFooBar).String()).To(Equal("http://example.com/api/experiments/foo_bar?foo=bar"))
	g.Expect(ep.Resolve(expFooBarTrials).String()).To(Equal("http://example.com/api/experiments/foo_bar/trials/?foo=bar"))
}
