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
	"context"
	"os/exec"

	"github.com/redskyops/redskyops-go/pkg/redskyapi"
)

// Config provides read-only access to the Red Sky configuration for commands
type Config interface {
	// Config is used to talk to the Red Sky API
	redskyapi.Config

	// SystemNamespace returns the Red Sky Controller system namespace (e.g. "redsky-system").
	SystemNamespace() (string, error)

	// Kubectl returns an executable command for running kubectl
	Kubectl(ctx context.Context, arg ...string) (*exec.Cmd, error)
}
