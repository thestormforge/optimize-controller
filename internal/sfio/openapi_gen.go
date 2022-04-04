//go:build ignore

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

// This program generates a stripped down version the Kubernetes swagger.json.
// You can use `go generate ./internal/sfio` to regenerate it.
package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/go-openapi/spec"
	"sigs.k8s.io/kustomize/kyaml/openapi/kubernetesapi"
)

func main() {
	// Load the default Swagger definition from the KYAML library
	var swagger spec.Swagger
	v := kubernetesapi.DefaultOpenAPI
	p := filepath.Join("kubernetesapi", v, "swagger.json")
	b := kubernetesapi.OpenAPIMustAsset[v](p)
	if err := swagger.UnmarshalJSON(b); err != nil {
		log.Fatal(err)
	}

	// Definitions are required but we don't need the descriptions
	for k, d := range swagger.Definitions {
		swagger.Definitions[k] = *d.WithDescription("")
		for n, p := range swagger.Definitions[k].Properties {
			swagger.Definitions[k].Properties[n] = *p.WithDescription("")
		}
	}

	// Paths are only used to determine if something has a namespace or not
	keepPaths := make(map[string]spec.PathItem, len(swagger.Paths.Paths))
	for path, pathInfo := range swagger.Paths.Paths {
		if pathInfo.Get == nil {
			continue
		}

		ext := pathInfo.Get.VendorExtensible
		gvk, found := ext.Extensions["x-kubernetes-group-version-kind"]
		if !found || ext.Extensions["x-kubernetes-action"] != "get" {
			continue
		}

		keepPaths[path] = spec.PathItem{
			PathItemProps: spec.PathItemProps{
				Get: &spec.Operation{VendorExtensible: spec.VendorExtensible{
					Extensions: map[string]interface{}{"x-kubernetes-group-version-kind": gvk},
				}},
			},
		}
	}
	swagger.Paths.Paths = keepPaths

	// Dump the result back out
	f, err := os.Create("swagger_min.json")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(&swagger); err != nil {
		log.Fatal(err)
	}
}
