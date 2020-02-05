/*
Copyright 2019 GramLabs, Inc.

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

// Package version is used to expose the current version information as populated
// by the build process. With a default value of "unreleased" the `BuildMetadata`
// indicates that `Version` will likely be used as _next_ Git tag. During a build
// some or all of the variables my be overridden using the Go linker.
package version

var (
	// Version is a "v" prefixed Semver
	Version = "v1.4.0"
	// BuildMetadata is the Semver build metadata stored independent of the version string
	BuildMetadata = "unreleased"
	// GitCommit is a Git commit identifier
	GitCommit = ""
)

// Info represents available version information
type Info struct {
	Version       string `json:"version"`
	BuildMetadata string `json:"build"`
	GitCommit     string `json:"gitCommit"`
}

// String returns the full Semver of the version information
func (i *Info) String() string {
	if i.Version == "" {
		return "v0.0.0"
	}
	if i.BuildMetadata == "" {
		return i.Version
	}
	return i.Version + "+" + i.BuildMetadata
}

// GetInfo returns the full version information
func GetInfo() *Info {
	return &Info{
		Version:       Version,
		BuildMetadata: BuildMetadata,
		GitCommit:     GitCommit,
	}
}
