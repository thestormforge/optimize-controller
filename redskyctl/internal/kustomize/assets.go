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
	"io"
)

// Create go run alias
// Path is relative to redskyctl/internal/kustomize
//go:generate -command assetGen go run ../../../cmd/generator/main.go --header ../../../hack/boilerplate.go.txt --package kustomize --path ../../../config/default

//go:generate assetGen

// Asset is a representation of an embedded file.
// The file will be gzip encoded via cmd/generator.
type Asset struct {
	data  []byte
	bytes []byte
}

var Assets = map[string]*Asset{
	"stock": &kustomizeBase,
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
		output bytes.Buffer
		zr     *gzip.Reader
	)

	// No need to decode again
	if len(a.bytes) > 0 {
		return nil
	}

	buf := bytes.NewBuffer(a.data)

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

func NewAssetFromBytes(b []byte) *Asset {
	return &Asset{
		bytes: b,
	}
}
