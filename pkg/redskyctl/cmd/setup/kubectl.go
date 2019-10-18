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

package setup

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/redskyops/k8s-experiment/pkg/controller/trial"
)

type Kubectl struct {
	Bin       string
	Namespace string

	// TODO Do we need to propagate other connection information to kubectl, just have a "globalArgs" array?
}

func (k *Kubectl) Complete() error {
	if k.Bin == "" {
		k.Bin = "kubectl"
	}
	return nil
}

// GenerateRedSkyOpsManifests returns a command that will produce the Red Sky Ops product manifests on stdout
func (k *Kubectl) GenerateRedSkyOpsManifests(env io.Reader) *exec.Cmd {
	args := []string{"run", "redsky-bootstrap"}

	// Create a single attached pod
	args = append(args, "--restart", "Never", "--attach")

	// Quietly remove the pod when we are done
	args = append(args, "--rm", "--quiet")

	// Use the image embedded in the code
	args = append(args, "--image", trial.Image)
	args = append(args, "--image-pull-policy", trial.ImagePullPolicy)

	// Overwrite the "redsky-system" namespace if configured
	if k.Namespace != "" {
		args = append(args, "--env", fmt.Sprintf("NAMESPACE=%s", k.Namespace))
	}

	// Tell kubectl to use stdin to read environment declarations
	if env != nil {
		args = append(args, "--stdin")
	}

	// Arguments passed to the container
	args = append(args, "--", "install")

	// Tell the installer to pick up environment declarations from stdin
	if env != nil {
		args = append(args, "-")
	}

	cmd := exec.Command(k.Bin, args...)
	if env != nil {
		cmd.Stdin = env
	}
	return cmd
}

// Apply returns a command that will apply the manifests specified on stdin
func (k *Kubectl) Apply() *exec.Cmd {
	args := []string{"apply"}

	// Take stdin
	args = append(args, "-f", "-")

	// TODO Handle upgrades
	//args = append(args, "--prune", "--selector", "app.kubernetes.io/name=redskyops,app.kubernetes.io/managed-by=%s")

	return exec.Command(k.Bin, args...)
}

// Create returns a command that will create the manifests specified on stdin
func (k *Kubectl) Create() *exec.Cmd {
	args := []string{"create"}

	// Take stdin
	args = append(args, "-f", "-")

	return exec.Command(k.Bin, args...)
}

// Delete returns a command that will delete the manifests specified on stdin
func (k *Kubectl) Delete() *exec.Cmd {
	args := []string{"delete"}

	// Take stdin
	args = append(args, "-f", "-")

	return exec.Command(k.Bin, args...)
}

// RunPiped runs c1 and pipes the output into c2
func RunPiped(c1, c2 *exec.Cmd) error {
	// TODO How do we get error text from either process?

	// TODO Or use os.Pipe, e.g. https://stackoverflow.com/questions/10781516/how-to-pipe-several-commands-in-go
	stdout, err := c1.StdoutPipe()
	if err != nil {
		return err
	}
	if err := c1.Start(); err != nil {
		return err
	}

	c2.Stdin = stdout
	if err := c2.Start(); err != nil {
		return err
	}

	if err := c1.Wait(); err != nil {
		return err
	}
	if err := c2.Wait(); err != nil {
		return err
	}
	return nil
}
