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

package main

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/binary"
	"errors"
	"math/rand"
	"net/http"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/thestormforge/optimize-controller/v2/cli/internal/commands"
	"github.com/thestormforge/optimize-controller/v2/internal/version"
	"golang.org/x/oauth2"
)

func init() {
	// Seed the pseudo random number generator using the cryptographic random number generator
	// https://stackoverflow.com/a/54491783
	var b [8]byte
	_, err := crypto_rand.Read(b[:])
	if err != nil {
		panic(err)
	}
	rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))

	// Prevent Cobra from changing the command order
	cobra.EnableCommandSorting = false
}

func main() {
	// Create a new root command
	cmd := commands.NewRootCommand()

	// TODO Include OS, etc. in comment?
	uaRoundTripper := version.UserAgent(cmd.Root().Name(), "", nil)

	// Generate a context which includes our UA string
	ctx := context.Background()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Transport: uaRoundTripper})

	// Run the command
	if err := cmd.ExecuteContext(ctx); err != nil {
		var e *exec.ExitError
		if errors.As(err, &e) && !e.Success() {
			os.Exit(e.ExitCode())
		}
		os.Exit(1)
	}
}
