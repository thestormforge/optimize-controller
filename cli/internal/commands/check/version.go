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
	"fmt"
	"net/url"
	"path"
	"runtime"

	"github.com/mmcdole/gofeed"
	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commander"
	"github.com/thestormforge/optimize-controller/v2/internal/version"
	"golang.org/x/mod/semver"
)

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
	feed, err := gofeed.NewParser().ParseURLWithContext("https://downloads.stormforge.io/stormforge-cli/index.xml", ctx)
	if err != nil {
		return fmt.Errorf("unable to find latest version")
	}

	versionInfo := version.GetInfo()
	for _, item := range feed.Items {
		if !semver.IsValid(item.Title) || semver.Prerelease(item.Title) != "" || semver.Compare(item.Title, versionInfo.Version) < 0 {
			continue
		}

		downloadURL, err := url.Parse(item.Link)
		if err != nil {
			return err
		}
		downloadURL.Path = path.Join(downloadURL.Path, fmt.Sprintf("stormforge_%s_%s_%s.tar.gz", item.Title, runtime.GOOS, runtime.GOARCH))

		_, _ = fmt.Fprintf(o.Out, "A newer version (%s) is available (you have %s)\n\n", item.Title, versionInfo.String())
		_, _ = fmt.Fprintf(o.Out, "Download the latest version:\n%s\n", downloadURL)
		return nil
	}

	_, _ = fmt.Fprintf(o.Out, "Version %s is the latest version\n", versionInfo.String())
	return nil
}
