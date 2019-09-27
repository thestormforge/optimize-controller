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

package util

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type ResourcePrinter interface {
	PrintObj(interface{}, io.Writer) error
}

type TableMeta interface {
	IsListType(obj interface{}) bool
	ExtractList(obj interface{}) ([]interface{}, error)
	ExtractValue(obj interface{}, column string) (string, error)
	Allow(outputFormat string) bool
	Columns(outputFormat string) []string
	Header(outputFormat string, column string) string
}

type NoPrinterError struct {
	OutputFormat   *string
	AllowedFormats []string
}

func (e NoPrinterError) Error() string {
	outputFormat := ""
	if e.OutputFormat != nil {
		outputFormat = *e.OutputFormat
	}
	sort.Strings(e.AllowedFormats)
	return fmt.Sprintf("no printer for %s, allowed formats are: %s", outputFormat, strings.Join(e.AllowedFormats, ","))
}

func IsNoPrinterError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(NoPrinterError)
	return ok
}

type JSONYAMLPrintFlags struct{}

func (f *JSONYAMLPrintFlags) AllowedFormats() []string {
	if f == nil {
		return []string{}
	}
	return []string{"json", "yaml"}
}

func (f *JSONYAMLPrintFlags) ToPrinter(outputFormat string) (ResourcePrinter, error) {
	switch strings.ToLower(outputFormat) {
	case "json":
		return &JSONPrinter{}, nil
	case "yaml":
		return &YAMLPrinter{}, nil
	default:
		return nil, NoPrinterError{OutputFormat: &outputFormat, AllowedFormats: f.AllowedFormats()}
	}
}

func (f *JSONYAMLPrintFlags) AddFlags(cmd *cobra.Command) {}

func NewJSONYAMLPrintFlags() *JSONYAMLPrintFlags {
	return &JSONYAMLPrintFlags{}
}

type TablePrintFlags struct {
	Meta       TableMeta
	Columns    []string
	NoHeader   *bool
	ShowLabels *bool
}

func (f *TablePrintFlags) AllowedFormats() []string {
	allowed := make([]string, 0)
	if f != nil && f.Meta != nil && f.Meta.Allow("name") {
		allowed = append(allowed, "name")
	}
	if f != nil && f.Meta != nil && f.Meta.Allow("wide") {
		allowed = append(allowed, "wide")
	}
	if f != nil && f.Meta != nil && f.Meta.Allow("csv") {
		allowed = append(allowed, "csv")
	}
	return allowed
}

func (f *TablePrintFlags) ToPrinter(outputFormat string) (ResourcePrinter, error) {
	if f.Meta == nil || !f.Meta.Allow(outputFormat) {
		return nil, NoPrinterError{OutputFormat: &outputFormat, AllowedFormats: f.AllowedFormats()}
	}

	headers := true
	if f.NoHeader != nil {
		headers = !(*f.NoHeader)
	}

	labels := false
	if f.ShowLabels != nil {
		labels = *f.ShowLabels
	}

	switch strings.ToLower(outputFormat) {
	case "", "wide":
		return &TablePrinter{meta: f.Meta, columns: f.Columns, headers: headers, labels: labels}, nil
	case "name":
		return &TablePrinter{meta: f.Meta, columns: []string{"name"}, outputFormat: "name", labels: labels}, nil
	case "csv":
		return &CSVPrinter{meta: f.Meta, headers: headers}, nil
	default:
		return nil, NoPrinterError{OutputFormat: &outputFormat, AllowedFormats: f.AllowedFormats()}
	}
}

func (f *TablePrintFlags) AddFlags(cmd *cobra.Command) {
	if f.NoHeader != nil {
		cmd.Flags().BoolVar(f.NoHeader, "no-headers", *f.NoHeader, "Don't print headers.")
	}
	if f.ShowLabels != nil {
		cmd.Flags().BoolVar(f.ShowLabels, "show-labels", *f.ShowLabels, "When printing, show all labels as the last column.")
	}
}

func NewTablePrintFlags(meta TableMeta) *TablePrintFlags {
	var nh, sl bool
	return &TablePrintFlags{
		Meta:       meta,
		NoHeader:   &nh,
		ShowLabels: &sl,
	}
}

type PrintFlags struct {
	JSONYAMLPrintFlags *JSONYAMLPrintFlags
	TablePrintFlags    *TablePrintFlags

	OutputFormat *string
}

func NewPrintFlags(meta TableMeta) *PrintFlags {
	return &PrintFlags{
		JSONYAMLPrintFlags: NewJSONYAMLPrintFlags(),
		TablePrintFlags:    NewTablePrintFlags(meta),
		OutputFormat:       stringptr(""),
	}
}

func (f *PrintFlags) AddFlags(cmd *cobra.Command) {
	f.JSONYAMLPrintFlags.AddFlags(cmd)
	f.TablePrintFlags.AddFlags(cmd)

	if f.OutputFormat != nil {
		cmd.Flags().StringVarP(f.OutputFormat, "output", "o", *f.OutputFormat, fmt.Sprintf("Output format. One of: %s", strings.Join(f.AllowedFormats(), "|")))
	}
}

func (f *PrintFlags) AllowedFormats() []string {
	var allowed []string
	allowed = append(allowed, f.JSONYAMLPrintFlags.AllowedFormats()...)
	allowed = append(allowed, f.TablePrintFlags.AllowedFormats()...)
	return allowed
}

func (f *PrintFlags) ToPrinter() (ResourcePrinter, error) {
	outputFormat := ""
	if f.OutputFormat != nil {
		outputFormat = *f.OutputFormat
	}

	if f.JSONYAMLPrintFlags != nil {
		if p, err := f.JSONYAMLPrintFlags.ToPrinter(outputFormat); !IsNoPrinterError(err) {
			return p, err
		}
	}

	if f.TablePrintFlags != nil {
		if p, err := f.TablePrintFlags.ToPrinter(outputFormat); !IsNoPrinterError(err) {
			return p, err
		}
	}

	return nil, NoPrinterError{OutputFormat: f.OutputFormat, AllowedFormats: f.AllowedFormats()}
}
