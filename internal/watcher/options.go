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
	"net/http"

	apps "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
)

type Option func(*watcher) error

func defaultOptions() *watcher {
	return &watcher{
		client:   &http.Client{},
		interval: 5,
	}
}

func WithAppCh(ch chan *apps.Application) Option {
	return func(w *watcher) (err error) {
		w.appCh = ch
		return
	}
}

func WithEndpoint(endpoint string) Option {
	return func(w *watcher) (err error) {
		w.endpoint = endpoint
		return
	}
}
