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

package kustomize

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
)

// Path is relative to redskyctl/internal/kustomize
//go:generate mkdir -p kustomizeTemp
//go:generate kustomize build ../../../config/default -o kustomizeTemp

// Create go run alias
//go:generate -command assetGen go run ../../../cmd/generator/main.go --header ../../../hack/boilerplate.go.txt --package kustomize

// Namespace
//go:generate assetGen --file kustomizeTemp/~g_v1_namespace_redsky-system.yaml --output namespace.go

// CRD
//go:generate assetGen --file kustomizeTemp/apiextensions.k8s.io_v1beta1_customresourcedefinition_trials.redskyops.dev.yaml --output trials.go
//go:generate assetGen --file kustomizeTemp/apiextensions.k8s.io_v1beta1_customresourcedefinition_experiments.redskyops.dev.yaml --output experiments.go

// RBAC
//go:generate assetGen --file kustomizeTemp/rbac.authorization.k8s.io_v1_clusterrolebinding_redsky-manager-rolebinding.yaml --output role_binding.go
//go:generate assetGen --file kustomizeTemp/rbac.authorization.k8s.io_v1_clusterrole_redsky-manager-role.yaml --output role.go

// Deployment
//go:generate assetGen --file kustomizeTemp/apps_v1_deployment_redsky-controller-manager.yaml --output deployment.go

// Cleanup
//go:generate rm -r kustomizeTemp

// Asset is a representation of an embedded file.
// The file will be gzipped and base64 encoded via cmd/generator.
type Asset struct {
	data  string
	bytes []byte
}

var Assets = map[string]Asset{
	"namespace":     Namespace,
	"experimentcrd": ExperimentCRD,
	"trialcrd":      TrialCRD,
	"managercrb":    ManagerClusterRoleBinding,
	"managercr":     ManagerClusterRole,
	"manager":       ManagerDeployment,
}

// Reader is a convenience function to provide an io.Reader interface for the
// embedded asset.
func (a *Asset) Reader() (io.Reader, error) {
	err := a.decode()
	return bytes.NewReader(a.bytes), err
}

// String provides a string representation of the embedded asset.
func (a *Asset) String() (string, error) {
	err := a.decode()

	return string(a.bytes), err
}

// Bytes provides the byte representation of the embedded asset.
func (a *Asset) Bytes() ([]byte, error) {
	err := a.decode()

	return a.bytes, err
}

func (a *Asset) decode() (err error) {
	var (
		decoded []byte
		output  bytes.Buffer
		zr      *gzip.Reader
	)

	// No need to decode again
	if len(a.bytes) > 0 {
		return nil
	}

	decoded, err = base64.StdEncoding.DecodeString(a.data)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(decoded)

	if zr, err = gzip.NewReader(buf); err != nil {
		return err
	}

	if _, err = io.Copy(&output, zr); err != nil {
		return err
	}

	if err = zr.Close(); err != nil {
		return err
	}

	a.bytes = output.Bytes()

	return nil
}
