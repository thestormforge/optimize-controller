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

package assets

//go:generate go run ../generator/generator.go --header ../../hack/boilerplate.go.txt --file ../../config/crd/bases/redskyops.dev_trials.yaml --package assets --output ./trials.go
//go:generate go run ../generator/generator.go --header ../../hack/boilerplate.go.txt --file ../../config/crd/bases/redskyops.dev_experiments.yaml --package assets --output ./experiments.go

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
)

type Asset struct {
	Data string
}

func (a *Asset) String() (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(a.Data)
	if err != nil {
		return "", err
	}

	buf := bytes.NewBuffer(decoded)
	zr, err := gzip.NewReader(buf)
	if err != nil {
		return "", err
	}

	var output bytes.Buffer
	if _, err := io.Copy(&output, zr); err != nil {
		return "", err
	}

	if err := zr.Close(); err != nil {
		return "", err
	}

	return output.String(), nil
}
