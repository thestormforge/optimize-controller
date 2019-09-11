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
package check

import (
	"fmt"
	"path"
)

// Linter is a collector of lint errors
type Linter interface {
	// For returns a new linter at the specified path
	For(elem ...interface{}) Linter
	// Severity determines how serious the lint is
	Severity(s int) Lint

	// Error is sugar for `Severity(0)`
	Error() Lint
	// Warning is sugar for `Severity(1)`
	Warning() Lint
}

// Lint is the general types of problems we can have
type Lint interface {
	WithDescription(description string) Lint
	Empty(thing string)
	Missing(thing string)
	Invalid(thing string, was interface{}, allowed ...interface{})
	Failed(thing string, err error)
}

// LintError is an indication that something is wrong
type LintError struct {
	Path        string
	Severity    int // 0 = error, 1 = warning, 2+ nitpicking...
	Message     string
	Description string
}

// TODO Expose some different formats
// TODO Make LintError sortable?

func (e *LintError) Error() string {
	return e.Message
}

type RootLinter struct {
	Problems []LintError
}

func NewLinter() *RootLinter {
	return &RootLinter{}
}

func (l *RootLinter) For(elem ...interface{}) Linter {
	ll := &lc{a: func(le LintError) { l.Problems = append(l.Problems, le) }}
	return ll.pp(elem...)
}

// Linter Context
type lc struct {
	a func(LintError)
	p string
	s int
	d string
}

var _ Linter = &lc{}
var _ Lint = &lc{}

func (l *lc) Empty(thing string) {
	l.aa("Missing %s", thing)
}

func (l *lc) Missing(thing string) {
	l.aa("Missing %s", thing)
}

func (l *lc) Invalid(thing string, was interface{}, allowed ...interface{}) {
	l.aa("Invalid %s: was '%s', expected one of %s", thing, was, allowed)
}

func (l *lc) Failed(thing string, err error) {
	l.aa("Invalid %s: %s", thing, err.Error())
}

func (l *lc) For(elem ...interface{}) Linter { ll := &lc{a: l.a, p: l.p}; return ll.pp(elem...) }
func (l *lc) WithDescription(d string) Lint  { l.d = d; return l }
func (l *lc) Severity(s int) Lint            { return &lc{p: l.p, a: l.a, s: s} }
func (l *lc) Error() Lint                    { return l.Severity(0) }
func (l *lc) Warning() Lint                  { return l.Severity(1) }
func (l *lc) aa(msg string, a ...interface{}) {
	l.a(LintError{Path: l.p, Severity: l.s, Message: fmt.Sprintf(msg, a...), Description: l.d})
}
func (l *lc) pp(elem ...interface{}) *lc {
	for _, e := range elem {
		switch v := e.(type) {
		case string:
			l.p = path.Join(l.p, v)
		case int:
			l.p = fmt.Sprintf("%s[%d]", l.p, v)
		}
	}
	return l
}
