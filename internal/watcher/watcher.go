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

package watcher

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	apps "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	"gopkg.in/yaml.v2"
)

// TODO
// Should this live in optimize-go ?
// I assume we'd want to use the same http client there to handle the auth/token
// bits.

type watcher struct {
	appCh chan *apps.Application

	client *http.Client
	// what endpoint (url) to watch
	endpoint string
	// how often we should check the endpoint ( seconds )
	interval int
}

func New(setters ...Option) (*watcher, error) {
	watch := defaultOptions()

	// Configure interface with any specified options
	for _, setter := range setters {
		if err := setter(watch); err != nil {
			return nil, err
		}
	}

	return watch, nil
}

func (w *watcher) Watch(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(w.interval) * time.Second)
	defer ticker.Stop()

	// TODO
	// This feels like it could be problematic ( n (all our customers) controllers
	// hitting some api every interval ). Some quick search shows we could look at
	// webhooks ( or similar), but they dont make sense in this situation because
	// the controller is not publicly accessible.
	// We could keep a long lived connection open, but this doesnt have a good feel
	// to it.
	// Not sure how I feel about a websocket, but might be an option

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			resp, err := w.client.Get(w.endpoint)
			if err != nil {
				log.Println(err)
			}

			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Println(err)
			}

			resp.Body.Close()

			myApp := &app.Application{}
			if err := yaml.Marshal(b, &myApp); err != nil {
				log.Println(err)
				continue
			}

			w.appCh <- myApp
		}
	}
}
