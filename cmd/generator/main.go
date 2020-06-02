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

// wtf. https://github.com/golang/tools/blob/master/go/analysis/passes/buildtag/buildtag.go#L47-L51
// // +build ignore

// Generator handles embedding non-go files into the compiled binary.
// Embedded assets are gzipped and base64 encoded.
// This makes use of redskyctl/internal/kustomize/assets.go to interact with the
// embedded data.
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"regexp"
	"strings"

	flag "github.com/spf13/pflag"
)

func main() {
	var (
		inputFiles    = flag.StringSlice("file", []string{}, "list of source files to include")
		outputPackage = flag.String("package", "generated", "output package name")
		outputFile    = flag.String("output", "output.go", "output filename")
		headerFile    = flag.String("header", "", "optional header file to include at the top of the generated file")
		prefix        = flag.String("prefix", "zz_generated", "output file prefix")
	)

	flag.Parse()

	if len(*inputFiles) == 0 {
		log.Fatal("file is required")
	}

	var outputBuffer bytes.Buffer
	if *headerFile != "" {
		h, err := ioutil.ReadFile(*headerFile)
		if err != nil {
			log.Fatal("failed to read header file")
		}
		outputBuffer.Write(h)
	}
	outputBuffer.WriteString("// Code generated, DO NOT EDIT.\n")
	outputBuffer.WriteString(fmt.Sprintf("package %s\n\n", *outputPackage))

	alphaNumRegEx, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		log.Fatal("idk why but this failed")
	}

	for _, inputFilename := range *inputFiles {
		var (
			variableName string
			ok           bool
		)

		if variableName, ok = filenameToVariablename[inputFilename]; !ok {
			// Auto generate variable name to be basename in camel case, ex:
			// redskyops.dev_trials.yaml => RedskyopsDevTrials
			// kustomization.yaml => Kustomization
			variableName = filepath.Base(inputFilename)
			variableName = strings.TrimSuffix(variableName, filepath.Ext(variableName))
			variableName = alphaNumRegEx.ReplaceAllString(variableName, " ")
			variableName = strings.Title(variableName)
			variableName = strings.Join(strings.Split(variableName, " "), "")
		}

		data, err := ioutil.ReadFile(inputFilename)
		if err != nil {
			log.Fatal("failed to read file", inputFilename)
		}

		// Compress data
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		zw.Name = filepath.Base(inputFilename)
		zw.Write(data)
		zw.Close()

		outputBuffer.WriteString("// The below is a gzipped + base64 encoded yaml\n")
		outputBuffer.WriteString(fmt.Sprintf("var %s = Asset{data: `%s`}\n", variableName, base64.StdEncoding.EncodeToString(buf.Bytes())))
	}

	if err := ioutil.WriteFile(strings.Join([]string{*prefix, *outputFile}, "."), outputBuffer.Bytes(), 0644); err != nil {
		log.Fatal("failed to write output file")
	}
}

var filenameToVariablename = map[string]string{
	"kustomizeTemp/~g_v1_namespace_redsky-system.yaml":                                                   "Namespace",
	"kustomizeTemp/apiextensions.k8s.io_v1beta1_customresourcedefinition_experiments.redskyops.dev.yaml": "ExperimentCRD",
	"kustomizeTemp/apiextensions.k8s.io_v1beta1_customresourcedefinition_trials.redskyops.dev.yaml":      "TrialCRD",
	"kustomizeTemp/rbac.authorization.k8s.io_v1_clusterrolebinding_redsky-manager-rolebinding.yaml":      "ManagerClusterRoleBinding",
	"kustomizeTemp/rbac.authorization.k8s.io_v1_clusterrole_redsky-manager-role.yaml":                    "ManagerClusterRole",
	"kustomizeTemp/apps_v1_deployment_redsky-controller-manager.yaml":                                    "ManagerDeployment"}
