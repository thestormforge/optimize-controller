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
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

type JSONPrinter struct{}

func (p *JSONPrinter) PrintObj(obj interface{}, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")
	err := enc.Encode(obj)
	return err
}

type YAMLPrinter struct{}

func (p *YAMLPrinter) PrintObj(obj interface{}, w io.Writer) error {
	output, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, string(output))
	return err
}

type TablePrinter struct {
	meta         TableMeta
	columns      []string
	headers      bool
	labels       bool
	outputFormat string
}

func (p *TablePrinter) PrintObj(obj interface{}, w io.Writer) error {
	var err error

	// Ensure we have a list of rows to iterate over
	var rows []interface{}
	if p.meta.IsListType(obj) {
		if rows, err = p.meta.ExtractList(obj); err != nil {
			return err
		}
		if len(rows) == 0 {
			_, err = fmt.Fprintln(w, "No resources found.")
			return err
		}
	} else {
		rows = []interface{}{obj}
	}

	// Ensure we have a list of column names
	columns := p.columns
	if len(columns) == 0 {
		columns = p.meta.Columns(p.outputFormat)
	}

	// Add labels if requested
	if p.labels {
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

func (p *TablePrinter) printRow(w io.Writer, row []string) error {
	if len(row) == 1 {
		// No trailing tab, no padding
		_, err := fmt.Fprintln(w, row[0])
		return err
	}

	_, err := fmt.Fprintf(w, "%s\t\n", strings.Join(row, "\t"))
	return err
}

type CSVPrinter struct {
	meta    TableMeta
	headers bool
}

func (p *CSVPrinter) PrintObj(obj interface{}, w io.Writer) error {
	var err error

	// Ensure we have a list of rows to iterate over
	var rows []interface{}
	if p.meta.IsListType(obj) {
		if rows, err = p.meta.ExtractList(obj); err != nil {
			return err
		}
	} else {
		rows = []interface{}{obj}
	}

	// Ensure we have a list of column names
	columns := p.meta.Columns("csv")

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

// This isn't a printer so much as it is an adaptor

// RuntimeObjectTableMeta is a subset of functionality needed for runtime.Object
type RuntimeObjectTableMeta interface {
	ExtractValue(obj runtime.Object, column string) (string, error)
	Allow(outputFormat string) bool
	Columns(outputFormat string) []string
	Header(outputFormat string, column string) string
}

func NewTableMeta(rt RuntimeObjectTableMeta) TableMeta {
	return &rtoTableMeta{tableMeta: rt}
}

type rtoTableMeta struct {
	tableMeta RuntimeObjectTableMeta
}

func (rtoTableMeta) IsListType(obj interface{}) bool {
	if o, ok := obj.(runtime.Object); ok {
		return meta.IsListType(o)
	}
	return false
}

func (rtoTableMeta) ExtractList(obj interface{}) ([]interface{}, error) {
	if o, ok := obj.(runtime.Object); ok {
		if meta.IsListType(o) {
			if l, err := meta.ExtractList(o); err != nil {
				return nil, err
			} else {
				ol := make([]interface{}, len(l))
				for i := range l {
					ol[i] = l[i]
				}
				return ol, nil
			}
		} else {
			return []interface{}{o}, nil
		}
	}
	return nil, fmt.Errorf("expected runtime.Object")
}

func (m *rtoTableMeta) ExtractValue(obj interface{}, column string) (string, error) {
	if o, ok := obj.(runtime.Object); ok {
		switch column {
		case "name":
			if acc, err := meta.Accessor(o); err != nil {
				return "", err
			} else {
				return acc.GetName(), nil
			}
		case "namespace":
			if acc, err := meta.Accessor(o); err != nil {
				return "", err
			} else {
				return acc.GetNamespace(), nil
			}
		case "labels":
			if acc, err := meta.Accessor(o); err != nil {
				return "", err
			} else {
				var l []string
				for k, v := range acc.GetLabels() {
					l = append(l, fmt.Sprintf("%s=%s", k, v))
				}
				return strings.Join(l, ","), nil
			}
		default:
			return m.tableMeta.ExtractValue(o, column)
		}
	}
	return "", fmt.Errorf("expected runtime.Object")
}

func (m *rtoTableMeta) Allow(outputFormat string) bool {
	return m.tableMeta.Allow(outputFormat)
}

func (m *rtoTableMeta) Columns(outputFormat string) []string {
	return m.tableMeta.Columns(outputFormat)
}

func (m *rtoTableMeta) Header(outputFormat string, column string) string {
	return m.tableMeta.Header(outputFormat, column)
}
