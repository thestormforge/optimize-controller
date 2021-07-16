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

package check

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/internal/version"
)

// TODO Should we have the option to initiate the download?
// TODO Should we also check the controller version and tell them to `init` to make the versions match?

var GitHubReleasesURL = "https://api.github.com/repos/thestormforge/optimize-controller/releases"

// VersionOptions are the options for checking the current version of the product
type VersionOptions struct {
	// IOStreams are used to access the standard process streams
	commander.IOStreams
}

// NewVersionCommand creates a new command for checking the current version of the product
func NewVersionCommand(o *VersionOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Check for the latest version number",
		Long:  "Check the current version number against the latest",

		PreRun: commander.StreamsPreRun(&o.IOStreams),
		RunE:   commander.WithContextE(o.checkVersion),
	}

	return cmd
}

func (o *VersionOptions) checkVersion(ctx context.Context) error {
	var releases ReleaseList
	if err := getJSON(ctx, GitHubReleasesURL, &releases); err != nil {
		return err
	}

	// Get the current version and strip any prerelease information
	versionInfo := version.GetInfo()
	cur := versionInfo.Version
	if i := strings.IndexRune(cur, '-'); i >= 0 {
		cur = cur[:i]
	}

	// Get the latest release
	latest := releases.Latest()
	if latest == nil {
		return fmt.Errorf("unable to find latest version")
	}

	// Just exit
	if latest.TagName == cur {
		_, _ = fmt.Fprintf(o.Out, "Version %s is the latest version\n", versionInfo.String())
		return nil
	}

	_, _ = fmt.Fprintf(o.Out, "A newer version (%s) is available (you have %s)\n", latest.Name, versionInfo.String())

	asset := latest.AssetByName(fmt.Sprintf("stormforge-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH))
	if asset != nil {
		_, _ = fmt.Fprintf(o.Out, "\nDownload the latest version:\n%s\n", asset.BrowserDownloadURL)
	}

	return nil
}

func getJSON(ctx context.Context, url string, obj interface{}) error {
	client := &http.Client{Transport: version.UserAgent("StormForgeOptimize", "", nil)}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body := json.NewDecoder(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		err := &APIError{StatusCode: resp.StatusCode}
		_ = body.Decode(err)
		return err
	}
	return body.Decode(obj)
}

type ReleaseList []Release

type Release struct {
	Name       string  `json:"name"`
	TagName    string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type APIError struct {
	StatusCode       int    `json:"-"`
	Message          string `json:"message"`
	DocumentationURL string `json:"documentation_url"`
}

func (rl ReleaseList) Latest() *Release {
	for i := range rl {
		if rl[i].Draft || rl[i].Prerelease {
			continue
		}
		return &rl[i]
	}
	return nil
}

func (r *Release) AssetByName(name string) *Asset {
	for i := range r.Assets {
		if r.Assets[i].Name == name {
			return &r.Assets[i]
		}
	}
	return nil
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("unexpected response (%d)", e.StatusCode)
}
