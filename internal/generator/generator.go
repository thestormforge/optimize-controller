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

// pacakge main does something
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
	outputBuffer.WriteString("// Code generated, do not edit manually.\n")
	outputBuffer.WriteString(fmt.Sprintf("package %s\n\n", *outputPackage))
	//outputBuffer.WriteString(assetWrapper + "\n")

	alphaNumRegEx, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		log.Fatal("idk why but this failed")
	}

	for _, inputFilename := range *inputFiles {
		// Auto generate variable name to be basename in camel case, ex:
		// redskyops.dev_trials.yaml => RedskyopsDevTrials
		// kustomization.yaml => Kustomization
		variableName := filepath.Base(inputFilename)
		variableName = strings.TrimSuffix(variableName, filepath.Ext(variableName))
		variableName = alphaNumRegEx.ReplaceAllString(variableName, " ")
		variableName = strings.Title(variableName)
		variableName = strings.Join(strings.Split(variableName, " "), "")

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

		outputBuffer.WriteString(fmt.Sprintf("var %s = Asset{data: `%s`}\n", variableName, base64.StdEncoding.EncodeToString(buf.Bytes())))
	}

	if err := ioutil.WriteFile(*outputFile, outputBuffer.Bytes(), 0644); err != nil {
		log.Fatal("failed to write output file")
	}
}
