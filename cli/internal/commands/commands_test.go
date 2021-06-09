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

package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestUsage(t *testing.T) {
	testCommandUsage(t, NewRootCommand())
}

func testCommandUsage(t *testing.T, cmd *cobra.Command) {
	// The overall goal here is to be consistent, and that includes considering the Cobra
	// generated commands and flags (like help).

	// For short descriptions (e.g. " help  Help about any command") we want sentence case
	// without the period. We also want to prevent wrapping on an 80 column layout so we
	// limit the length.

	// For flags (e.g. "-h, --help  help for xxx") we want lower case without the period.

	t.Run(cmd.Name(), func(t *testing.T) {
		// Short description
		fw := strings.Fields(cmd.Short)[0]
		assert.Equal(t, strings.Title(fw), fw)
		assert.False(t, strings.HasSuffix(cmd.Short, "."))
		assert.Greater(t, 60, len(cmd.Short))

		// TODO If cmd.Args is set, check that Name() != Use

		// Flags
		cmd.Flags().VisitAll(func(f *flag.Flag) {
			t.Run("--"+f.Name, func(t *testing.T) {
				assert.False(t, strings.HasSuffix(cmd.Short, "."))

				if _, u := flag.UnquoteUsage(f); assert.NotEmpty(t, u) {
					fw := strings.Fields(u)[0]
					assert.Equal(t, normalizeFlagUsage(fw), fw)
					//	assert.NotEqual(t, "string", n)
				}

				if f.Name == "filename" {
					assert.Containsf(t, f.Annotations, cobra.BashCompFilenameExt, "cmd.MarkFlagFilename('filename', 'ext'...) missing")
				}

				if strings.Contains(f.Usage, "one of") {
					assert.Regexp(t, ".*; one of: (.+)(|.+)*", f.Usage)
				}
			})
		})

		// Recurse into the children commands
		for _, c := range cmd.Commands() {
			testCommandUsage(t, c)
		}
	})
}

func normalizeFlagUsage(usageFirstWord string) string {
	if usageFirstWord == "Kustomize" {
		return usageFirstWord
	}
	return strings.ToLower(usageFirstWord)
}
