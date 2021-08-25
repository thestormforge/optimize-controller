/*
Copyright 2021 GramLabs, Inc.

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

package sfio

import (
	_ "embed"

	"sigs.k8s.io/kustomize/kyaml/openapi"
)

//go:generate go run openapi_gen.go

//go:embed swagger_min.json
var customSchema []byte

func init() {
	// The full Kubernetes schema embedded into KYAML is over 5MB and takes
	// considerable time and memory to parse. By using a stripped down version
	// of the same schema we can achieve considerable savings.
	// NOTE: This issue may be been resolved in later versions of KYAML
	err := openapi.SetSchema(nil, customSchema, true)
	if err != nil {
		panic(err)
	}
}
