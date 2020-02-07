// Copyright 2016 The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file originally comes from the Prometheus Operator project:
// https://github.com/coreos/prometheus-operator/blob/master/cmd/po-docgen/api.go
// Modifications have been made to account for using this code in this project,
// the original file is available in Git history.

package docs

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"io"
	"reflect"
	"strings"
)

const (
	firstParagraph = `
# API Docs

`
)

var (
	links = map[string]string{
		"batchv1beta1.JobTemplateSpec": "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#jobtemplatespec-v1beta1-batch",
		"corev1.ObjectReference":       "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectreference-v1-core",
		"corev1.Volume":                "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#volume-v1-core",
		"corev1.VolumeMount":           "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#volumemount-v1-core",
		"metav1.ObjectMeta":            "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectmeta-v1-meta",
		"metav1.ListMeta":              "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#listmeta-v1-meta",
		"metav1.LabelSelector":         "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#labelselector-v1-meta",
		"metav1.Time":                  "https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#time-v1-meta",
		//"metav1.Duration":      "#unknown",
	}

	selfLinks = map[string]string{}
)

func toSectionLink(name string) string {
	name = strings.ToLower(name)
	name = strings.Replace(name, " ", "-", -1)
	return name
}

func printTOC(out io.Writer, types []KubeTypes) {
	_, _ = fmt.Fprintf(out, "\n## Table of Contents\n")
	for _, t := range types {
		strukt := t[0]
		_, _ = fmt.Fprintf(out, "* [%s](#%s)\n", strukt.Name, toSectionLink(strukt.Name))
	}
}

func printAPIDocs(out io.Writer, path string) {
	_, _ = fmt.Fprintln(out, firstParagraph)

	types := ParseDocumentationFrom(path)
	for _, t := range types {
		strukt := t[0]
		selfLinks[strukt.Name] = "#" + strings.ToLower(strukt.Name)
	}

	// we need to parse once more to now add the self links
	types = ParseDocumentationFrom(path)

	printTOC(out, types)

	for _, t := range types {
		strukt := t[0]
		_, _ = fmt.Fprintf(out, "\n## %s\n\n%s\n\n", strukt.Name, strukt.Doc)

		_, _ = fmt.Fprintln(out, "| Field | Description | Scheme | Required |")
		_, _ = fmt.Fprintln(out, "| ----- | ----------- | ------ | -------- |")
		fields := t[1:(len(t))]
		for _, f := range fields {
			_, _ = fmt.Fprintln(out, "|", fmt.Sprintf("`%s`", f.Name), "|", f.Doc, "|", fmt.Sprintf("_%s_", f.Type), "|", f.Mandatory, "|")
		}
		if len(fields) == 0 {
			_, _ = fmt.Fprintln(out, "| _N/A_ |")
		}
		_, _ = fmt.Fprintln(out, "")
		_, _ = fmt.Fprintln(out, "[Back to TOC](#table-of-contents)")
	}
}

// Pair of strings. We keed the name of fields and the doc
type Pair struct {
	Name, Doc, Type string
	Mandatory       bool
}

// KubeTypes is an array to represent all available types in a parsed file. [0] is for the type itself
type KubeTypes []Pair

// ParseDocumentationFrom gets all types' documentation and returns them as an
// array. Each type is again represented as an array (we have to use arrays as we
// need to be sure for the order of the fields). This function returns fields and
// struct definitions that have no documentation as {name, ""}.
func ParseDocumentationFrom(src string) []KubeTypes {
	var docForTypes []KubeTypes

	pkg := astFrom(src)

	for _, kubType := range pkg.Types {
		if structType, ok := kubType.Decl.Specs[0].(*ast.TypeSpec).Type.(*ast.StructType); ok {
			var ks KubeTypes
			ks = append(ks, Pair{kubType.Name, fmtRawDoc(kubType.Doc), "", false})

			for _, field := range structType.Fields.List {
				typeString := fieldType(field.Type)
				fieldMandatory := fieldRequired(field)
				if n := fieldName(field); n != "-" {
					fieldDoc := fmtRawDoc(field.Doc.Text())
					ks = append(ks, Pair{n, fieldDoc, typeString, fieldMandatory})
				}
			}
			docForTypes = append(docForTypes, ks)
		}
	}

	return docForTypes
}

func astFrom(filePath string) *doc.Package {
	fset := token.NewFileSet()
	m := make(map[string]*ast.File)

	f, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	m[filePath] = f
	apkg, _ := ast.NewPackage(fset, m, nil, nil)

	return doc.New(apkg, "", 0)
}

func fmtRawDoc(rawDoc string) string {
	var buffer bytes.Buffer
	delPrevChar := func() {
		if buffer.Len() > 0 {
			buffer.Truncate(buffer.Len() - 1) // Delete the last " " or "\n"
		}
	}

	// Ignore all lines after ---
	rawDoc = strings.Split(rawDoc, "---")[0]

	for _, line := range strings.Split(rawDoc, "\n") {
		line = strings.TrimRight(line, " ")
		leading := strings.TrimLeft(line, " ")
		switch {
		case len(line) == 0: // Keep paragraphs
			delPrevChar()
			buffer.WriteString("\n\n")
		case strings.HasPrefix(leading, "TODO"): // Ignore one line TODOs
		case strings.HasPrefix(leading, "+"): // Ignore instructions to go2idl
		default:
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				delPrevChar()
				line = "\n" + line + "\n" // Replace it with newline. This is useful when we have a line with: "Example:\n\tJSON-someting..."
			} else {
				line += " "
			}
			buffer.WriteString(line)
		}
	}

	postDoc := strings.TrimRight(buffer.String(), "\n")
	//postDoc = strings.Replace(postDoc, "\\\"", "\"", -1) // replace user's \" to "
	//postDoc = strings.Replace(postDoc, "\"", "\\\"", -1) // Escape "
	postDoc = strings.Replace(postDoc, "\n", "\\n", -1)
	postDoc = strings.Replace(postDoc, "\t", "\\t", -1)
	postDoc = strings.Replace(postDoc, "|", "\\|", -1)

	return postDoc
}

func toLink(typeName string) string {
	selfLink, hasSelfLink := selfLinks[typeName]
	if hasSelfLink {
		return wrapInLink(typeName, selfLink)
	}

	link, hasLink := links[typeName]
	if hasLink {
		return wrapInLink(typeName, link)
	}

	return typeName
}

func wrapInLink(text, link string) string {
	parts := strings.Split(text, ".")
	if len(parts) > 0 {
		text = parts[len(parts)-1]
	}
	return fmt.Sprintf("[%s](%s)", text, link)
}

// fieldName returns the name of the field as it should appear in JSON format
// "-" indicates that this field is not part of the JSON representation
func fieldName(field *ast.Field) string {
	jsonTag := ""
	if field.Tag != nil {
		jsonTag = reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1]).Get("json") // Delete first and last quotation
		if strings.Contains(jsonTag, "inline") {
			return "-"
		}
	}

	jsonTag = strings.Split(jsonTag, ",")[0] // This can return "-"
	if jsonTag == "" {
		if field.Names != nil {
			return field.Names[0].Name
		}
		return field.Type.(*ast.Ident).Name
	}
	return jsonTag
}

// fieldRequired returns whether a field is a required field.
func fieldRequired(field *ast.Field) bool {
	jsonTag := ""
	if field.Tag != nil {
		jsonTag = reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1]).Get("json") // Delete first and last quotation
		return !strings.Contains(jsonTag, "omitempty")
	}

	return false
}

func fieldType(typ ast.Expr) string {
	switch typ.(type) {
	case *ast.Ident:
		return toLink(typ.(*ast.Ident).Name)
	case *ast.StarExpr:
		return "*" + toLink(fieldType(typ.(*ast.StarExpr).X))
	case *ast.SelectorExpr:
		e := typ.(*ast.SelectorExpr)
		pkg := e.X.(*ast.Ident)
		t := e.Sel
		return toLink(pkg.Name + "." + t.Name)
	case *ast.ArrayType:
		return "[]" + toLink(fieldType(typ.(*ast.ArrayType).Elt))
	case *ast.MapType:
		mapType := typ.(*ast.MapType)
		return "map[" + toLink(fieldType(mapType.Key)) + "]" + toLink(fieldType(mapType.Value))
	default:
		return ""
	}
}
