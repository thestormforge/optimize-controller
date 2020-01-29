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

// Loader is used to initially populate a client configuration
type Loader func(cfg *ClientConfig) error

// Change is used to apply a configuration change that should be persisted
type Change func(cfg *Config) error

// ClientConfig is the structure used to manage configuration data
type ClientConfig struct {
	// Filename is the path to the configuration file; if left blank, it will be populated using XDG base directory conventions on the next Load
	Filename string

	data        Config
	unpersisted []Change
}

// Load will populate the client configuration
func (cc *ClientConfig) Load(extra ...Loader) error {
	var loaders []Loader
	loaders = append(loaders, extra...)
	for i := range loaders {
		if err := loaders[i](cc); err != nil {
			return err
		}
	}
	return nil
}

// Update will make a change to the configuration data that should be persisted on the next call to Write
func (cc *ClientConfig) Update(change Change) error {
	if err := change(&cc.data); err != nil {
		return err
	}
	cc.unpersisted = append(cc.unpersisted, change)
	return nil
}
