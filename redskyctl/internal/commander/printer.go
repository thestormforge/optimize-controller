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

package commander

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"encoding/json"

	redskyv1alpha1 "github.com/redskyops/redskyops-controller/pkg/apis/redsky/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/duration"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/yaml"
)

const (
	// PrinterAllowedFormats is the configuration key for
	PrinterAllowedFormats = "allowedFormats"
	// PrinterOutputFormat is the configuration key for setting the initial output format
	PrinterOutputFormat = "outputFormat"
	// PrinterColumns is the configuration key for setting the initial column list
	PrinterColumns = "columns"
	// PrinterNoHeader is the configuration key for setting the initial suppress header flag
	PrinterNoHeader = "noHeader"
	// PrinterShowLabels is the configuration key for setting the initial show labels flag
	PrinterShowLabels = "showLabels"
	// PrinterHideStatus
	PrinterHideStatus = "hideStatus"
)

// ResourcePrinter formats an object to a byte stream
type ResourcePrinter interface {
	// PrintObj formats the specified object to the specified writer
	PrintObj(interface{}, io.Writer) error
}

// TableMeta is used to inspect objects for formatting
type TableMeta interface {
	// ExtractList accepts a single object (which possibly represents a list) and returns a slice to iterate over; this
	// should include a single element slice from the input object if it does not represent a list
	ExtractList(obj interface{}) ([]interface{}, error)
	// Columns returns the default list of columns to render for a given object (in some cases this may be overridden by the user)
	Columns(obj interface{}, outputFormat string) []string
	// ExtractValue returns the column string value for a given object from the extract list result
	ExtractValue(obj interface{}, column string) (string, error)
	// Header returns the header value to use for a column
	Header(outputFormat string, column string) string
}

// NoPrinterError is an error occurring when no suitable printer is available
type NoPrinterError struct {
	// OutputFormat is the requested output format
	OutputFormat string
	// AllowedFormats are the available output formats
	AllowedFormats []string
}

// Error returns a useful message for a "no printer" error
func (e NoPrinterError) Error() string {
	sort.Strings(e.AllowedFormats)
	return fmt.Sprintf("no printer for %s, allowed formats are: %s", e.OutputFormat, strings.Join(e.AllowedFormats, ","))
}

// requiresMeta returns true for the formats that require a TableMeta
func requiresMeta(outputFormat string) bool {
	switch outputFormat {
	case "name", "wide", "csv", "":
		return true
	}
	return false
}

// printFlags are the options for creating a printer
type printFlags struct {
	// allowedFormats are the possible formats
	allowedFormats []string
	// outputFormat determines what type of printer should be created
	outputFormat string
	// meta is an optional inspector required for some formats
	meta TableMeta
	// columns overrides the default column list for supported formats
	columns []string
	// noHeader suppresses the headers for supported formats
	noHeader bool
	// showLabels includes labels in supported formats
	showLabels bool
}

// printFlagsFieldSep checks for the field separator when parsing configuration values
func printFlagsFieldSep(c rune) bool { return c == ',' }

// newPrintFlags returns a new print flags instance with settings parsed from a map of name/value configuration pairs
func newPrintFlags(meta TableMeta, config map[string]string) *printFlags {
	pf := &printFlags{meta: meta}

	// Hide status of Kube objects
	if kp, ok := meta.(*kubePrinter); ok {
		kp.hideStatus, _ = strconv.ParseBool(config[PrinterHideStatus])
	}

	// Split the column list
	pf.columns = strings.FieldsFunc(config[PrinterColumns], printFlagsFieldSep)
	for i := range pf.columns {
		pf.columns[i] = strings.TrimSpace(pf.columns[i])
	}

	// Parse boolean flags (ignore failures and leave false)
	pf.noHeader, _ = strconv.ParseBool(config[PrinterNoHeader])
	pf.showLabels, _ = strconv.ParseBool(config[PrinterShowLabels])

	// Compute the list of allowed printer formats
	outputFormat := strings.ToLower(config[PrinterOutputFormat])
	allowedFormats := strings.FieldsFunc(config[PrinterAllowedFormats], printFlagsFieldSep)
	for i := range allowedFormats {
		allowedFormats[i] = strings.ToLower(strings.TrimSpace(allowedFormats[i]))
	}
	if len(allowedFormats) == 0 {
		allowedFormats = []string{"json", "yaml", "name", "wide", "csv", ""}
	}

	for _, allowedFormat := range allowedFormats {
		if requiresMeta(allowedFormat) && pf.meta == nil {
			continue
		}
		pf.allowedFormats = append(pf.allowedFormats, allowedFormat)

		// Only set the output format if it is allowed
		if outputFormat == allowedFormat {
			pf.outputFormat = allowedFormat
		}
	}

	// Override the output format if there is no choice
	if len(pf.allowedFormats) == 1 {
		pf.outputFormat = pf.allowedFormats[0]
	}

	return pf
}

// addFlags adds command line flags for configuring the printer
func (f *printFlags) addFlags(cmd *cobra.Command) {
	// We only need an output flag if there is a choice
	if len(f.allowedFormats) > 1 {
		allowed := strings.Trim(strings.ReplaceAll(strings.Join(f.allowedFormats, "|"), "||", "|"), "|")
		cmd.Flags().StringVarP(&f.outputFormat, "output", "o", f.outputFormat, fmt.Sprintf("Output `format`. One of: %s", allowed))
	}

	// These flags only work with formats that require metadata, make sure we have at least one
	for _, allowedFormat := range f.allowedFormats {
		if requiresMeta(allowedFormat) {
			cmd.Flags().BoolVar(&f.noHeader, "no-headers", f.noHeader, "Don't print headers.")
			cmd.Flags().BoolVar(&f.showLabels, "show-labels", f.showLabels, "When printing, show all labels as the last column.")
			break
		}
	}
}

// toPrinter generates a new printer
func (f *printFlags) toPrinter(printer *ResourcePrinter) error {
	outputFormat := strings.ToLower(f.outputFormat)
	for _, allowedFormat := range f.allowedFormats {
		if outputFormat == allowedFormat {
			switch outputFormat {
			case "json", "yaml":
				*printer = &marshalPrinter{outputFormat: outputFormat}
				return nil
			case "wide", "":
				*printer = &tablePrinter{meta: f.meta, columns: f.columns, headers: !f.noHeader, showLabels: f.showLabels}
				return nil
			case "name":
				*printer = &tablePrinter{meta: f.meta, columns: []string{"name"}, outputFormat: outputFormat, showLabels: f.showLabels}
				return nil
			case "csv":
				*printer = &csvPrinter{meta: f.meta, headers: !f.noHeader}
				return nil
			}
		}
	}
	return NoPrinterError{OutputFormat: f.outputFormat, AllowedFormats: f.allowedFormats}
}

// marshalPrinter is a printer that generates output using some type of generic encoding (e.g. JSON)
type marshalPrinter struct {
	// outputFormat is the name of the marshaller to use, JSON will be used if it is unrecognized
	outputFormat string
}

// PrintObj will marshal the supplied object
func (p *marshalPrinter) PrintObj(obj interface{}, w io.Writer) error {
	if strings.ToLower(p.outputFormat) == "yaml" {
		output, err := yaml.Marshal(obj)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, string(output))
		return err
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")
	err := enc.Encode(obj)
	return err
}

// tablePrinter is a printer that generates tabular output
type tablePrinter struct {
	// meta is used to extract information about the objects being formatted
	meta TableMeta
	// columns is the list of columns to generate
	columns []string
	// headers determines if the header row should be included
	headers bool
	// showLabels determines if the "labels" column should be included
	showLabels bool
	// outputFormat is the format this printer is generating (used to alter defaults)
	outputFormat string
}

// PrintObj generates the tabular data
func (p *tablePrinter) PrintObj(obj interface{}, w io.Writer) error {
	// Ensure we have a list of rows to iterate over
	rows, err := p.meta.ExtractList(obj)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		_, err = fmt.Fprintln(w, "No resources found.")
		return err
	}

	// Ensure we have a list of column names
	columns := p.columns
	if len(columns) == 0 {
		columns = p.meta.Columns(obj, p.outputFormat)
	}

	// Add labels if requested
	if p.showLabels {
		columns = append(columns, "labels")
	}

	// Allocate a tab writer and a row buffer
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	buf := make([]string, len(columns))

	// Print headers
	if p.headers {
		for i := range columns {
			buf[i] = p.meta.Header(p.outputFormat, columns[i])
		}
		if err = p.printRow(tw, buf); err != nil {
			return err
		}
	}

	// Print data
	for y := range rows {
		for x := range columns {
			buf[x], err = p.meta.ExtractValue(rows[y], columns[x])
			if err != nil {
				return err
			}
		}
		if err = p.printRow(tw, buf); err != nil {
			return err
		}
	}

	return tw.Flush()
}

// printRow formats a single row
func (p *tablePrinter) printRow(w io.Writer, row []string) error {
	if len(row) == 1 {
		// No trailing tab, no padding
		_, err := fmt.Fprintln(w, row[0])
		return err
	}

	_, err := fmt.Fprintf(w, "%s\t\n", strings.Join(row, "\t"))
	return err
}

// csvPrinter generates Comma Separated Value (CSV) output
type csvPrinter struct {
	// meta is used to extract information about the objects being formatted
	meta TableMeta
	// headers determines if the header row should be included
	headers bool
}

// PrintObj generates the CSV data
func (p *csvPrinter) PrintObj(obj interface{}, w io.Writer) error {
	// Ensure we have a list of rows to iterate over
	rows, err := p.meta.ExtractList(obj)
	if err != nil {
		return err
	}

	// Ensure we have a list of column names
	columns := p.meta.Columns(obj, "csv")

	// Allocate a CSV writer and a record buffer
	cw := csv.NewWriter(w)
	buf := make([]string, len(columns))

	// Print headers
	if p.headers {
		for i := range columns {
			buf[i] = p.meta.Header("csv", columns[i])
		}
		if err = cw.Write(buf); err != nil {
			return err
		}
	}

	// Print data
	for y := range rows {
		for x := range columns {
			if buf[x], err = p.meta.ExtractValue(rows[y], columns[x]); err != nil {
				return err
			}
		}
		if err = cw.Write(buf); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
}

// kubePrinter handles both metadata extraction and printing of objects registered to an API Machinery scheme
type kubePrinter struct {
	printer    ResourcePrinter
	hideStatus bool
}

// ExtractList returns a slice of the items from a Kube list object
func (k *kubePrinter) ExtractList(obj interface{}) ([]interface{}, error) {
	// Must be a runtime object
	o, ok := obj.(runtime.Object)
	if !ok {
		return nil, fmt.Errorf("expected runtime.Object")
	}

	// If it's not a list, just wrap the single element
	if !meta.IsListType(o) {
		return []interface{}{o}, nil
	}

	// Extract the actual "items"
	l, err := meta.ExtractList(o)
	if err != nil {
		return nil, err
	}

	// Change type
	ll := make([]interface{}, len(l))
	for i := range l {
		ll[i] = l[i]
	}
	return ll, nil
}

// Columns just returns a fixed set of columns
func (k *kubePrinter) Columns(obj interface{}, outputFormat string) []string {
	// TODO Can we inspect the object reflectively for print columns?
	return []string{"name", "age"}
}

// ExtractValue attempts to extract common columns from a Kube runtime object
func (k *kubePrinter) ExtractValue(obj interface{}, column string) (string, error) {
	o, ok := obj.(runtime.Object)
	if !ok {
		return "", fmt.Errorf("expected runtime.Object")
	}

	switch column {
	case "name":
		acc, err := meta.Accessor(o)
		if err != nil {
			return "", err
		}
		return acc.GetName(), nil

	case "namespace":
		acc, err := meta.Accessor(o)
		if err != nil {
			return "", err
		}
		return acc.GetNamespace(), nil

	case "age":
		acc, err := meta.Accessor(o)
		if err != nil {
			return "", err
		}
		timestamp := acc.GetCreationTimestamp()
		if timestamp.IsZero() {
			return "<unknown>", nil
		}
		return duration.HumanDuration(time.Since(timestamp.Time)), nil

	case "labels":
		acc, err := meta.Accessor(o)
		if err != nil {
			return "", err
		}
		var l []string
		for k, v := range acc.GetLabels() {
			l = append(l, fmt.Sprintf("%s=%s", k, v))
		}
		return strings.Join(l, ","), nil
	}

	return "", fmt.Errorf("unable to extract: %s", column)
}

// Header formats all headers in upper case
func (k *kubePrinter) Header(outputFormat string, column string) string {
	return strings.ToUpper(column)
}

// PrintObj converts the Kube runtime object to an unstructured object for marshalling
func (k *kubePrinter) PrintObj(obj interface{}, w io.Writer) error {
	// As a special case, deal with runtime.Object, it's type metadata, and it's silly creation timestamp
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = redskyv1alpha1.AddToScheme(scheme)
	u := &unstructured.Unstructured{}
	// TODO Is the InternalGroupVersioner going to cause issues based on the version of client-go we use?
	if err := scheme.Convert(obj, u, runtime.InternalGroupVersioner); err == nil {
		u.SetCreationTimestamp(metav1.Time{})
		if k.hideStatus {
			delete(u.UnstructuredContent(), "status")
		}

		obj = u
	}
	return k.printer.PrintObj(obj, w)
}
