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

package util

import (
	"fmt"
	"os"
	"os/exec"

	redskyapi "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/spf13/cobra"
)

// CheckErr ensures the supplied error is reported and handled correctly. Errors may be reported and/or may cause the process to exit.
func CheckErr(cmd *cobra.Command, err error) {
	if err == nil {
		return
	}

	// Handle forked process errors by propagating the exit status
	if eerr, ok := err.(*exec.ExitError); ok && !eerr.Success() {
		os.Exit(eerr.ExitCode())
	}

	// Handle unauthorized errors by suggesting `login`
	if redskyapi.IsUnauthorized(err) {
		msg := "unauthorized"
		if _, ok := err.(*redskyapi.Error); ok {
			msg = err.Error()
		}
		err = fmt.Errorf("%s, try running 'redskyctl login'", msg)
	}

	// TODO With the exception of silence usage behavior and stdout vs. stderr, this is basically what Cobra already does with a RunE...
	cmd.PrintErrln("Error:", err.Error())
	os.Exit(1)
}

func stringptr(val string) *string {
	return &val
}
