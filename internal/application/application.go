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

package application

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	"github.com/thestormforge/optimize-controller/internal/scan"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
)

// FilterScenarios retains only the named scenario on the supplied application. Removing unused
// scenarios may be useful for some types of application operations. If the requested scenario
// cannot be found, an error is returned.
func FilterScenarios(app *redskyappsv1alpha1.Application, scenario string) error {
	if scenario == "" {
		if len(app.Scenarios) > 1 {
			names := make([]string, 0, len(app.Scenarios))
			for _, s := range app.Scenarios {
				names = append(names, s.Name)
			}
			return fmt.Errorf("scenario is required (should be one of '%s')", strings.Join(names, "', '"))
		}
		return nil
	}

	switch len(app.Scenarios) {

	case 0:
		return fmt.Errorf("unknown scenario '%s' (application has no scenarios defined)", scenario)

	case 1:
		if cleanName(app.Scenarios[0].Name) == scenario {
			return nil
		}
		return fmt.Errorf("unknown scenario '%s' (must be '%s')", scenario, app.Scenarios[0].Name)

	default:
		names := make([]string, 0, len(app.Scenarios))
		for i := range app.Scenarios {
			if cleanName(app.Scenarios[i].Name) == scenario {
				// Only keep the requested scenario
				app.Scenarios = app.Scenarios[i : i+1]
				return nil
			}
			names = append(names, app.Scenarios[i].Name)
		}
		return fmt.Errorf("unknown scenario '%s' (should be one of '%s')", scenario, strings.Join(names, "', '"))

	}
}

// FilterObjectives retains and re-orders the named scenarios on the supplied application. Removing
// unused objectives may be useful for some types of application operations. If the requested
// objectives cannot be found, an error is returned.
func FilterObjectives(app *redskyappsv1alpha1.Application, objectives []string) error {
	// No filter, keep all objectives
	if len(objectives) == 0 {
		return nil
	}

	// Keep will have the same explicit order as the requested objectives
	keep := make([]redskyappsv1alpha1.Objective, 0, len(objectives))
	unknown := make([]string, 0, len(objectives))

FOUND:
	for _, name := range objectives {
		for i := range app.Objectives {
			if cleanName(app.Objectives[i].Name) == name {
				keep = append(keep, app.Objectives[i])
				continue FOUND
			}
		}
		unknown = append(unknown, name)
	}

	if len(keep) != cap(keep) {
		return fmt.Errorf("unknown objectives %s", strings.Join(unknown, ", "))
	}

	app.Objectives = keep
	return nil
}

// ExperimentName returns the name of an experiment corresponding to the application state. Before
// passing an application, be sure to filter scenarios and objectives.
func ExperimentName(application *redskyappsv1alpha1.Application) string {
	// Default the application to avoid empty names (deep copy first so we don't impact the caller)
	app := application.DeepCopy()
	app.Default()

	names := make([]string, 0, 1+len(app.Scenarios)+len(app.Objectives))
	names = append(names, app.Name)

	for _, s := range app.Scenarios {
		names = append(names, cleanName(s.Name))
	}

	if !application.HasDefaultObjectives() {
		for _, o := range app.Objectives {
			names = append(names, cleanName(o.Name))
		}
	}

	return strings.Join(names, "-")
}

// FilterByExperimentName filters the scenarios and objectives based on an experiment name.
// This can fail with an "ambiguous name" error if the combination of scenario and objective
// results in multiple possible combinations for the given experiment name. For example, if
// application name is "a", there are scenarios named "s" and "s-s" and objectives named
// "s-o" and "o" then the experiment name "a-s-s-o" could be "s" and "s-o" OR "s-s" and "o".
// Callers should have a back up plan for invoking `Filter*` methods independently.
func FilterByExperimentName(app *redskyappsv1alpha1.Application, name string) error {
	e := newLexer(app, name)

	// Eat the application name at the start (it will error if they don't match)
	if _, err := e.next(); err != nil {
		return err
	}

	// Get the scenario name and filter the application with it
	if scenario, err := e.next(); err != nil {
		return err
	} else if err := FilterScenarios(app, scenario); err != nil {
		return err
	}

	// Accumulate the objectives
	var objectives []string
	o, err := e.next()
	for err == nil {
		objectives = append(objectives, o)
		o, err = e.next()
	}
	if err != errEos {
		return err
	}
	return FilterObjectives(app, objectives)
}

// AmbiguousNameError is returned from `FilterByExperimentName` when an experiment name maps
// back to multiple combinations of scenario and objective names.
type AmbiguousNameError struct {
	Name string
}

// Error returns a description of the ambiguous name error.
func (e *AmbiguousNameError) Error() string {
	return fmt.Sprintf("ambiguous name '%s'", e.Name)
}

// WorkingDirectory returns the directory the application was loaded from. This
// directory should be used as the effective working directory when resolving relative
// paths found in the application definition.
func WorkingDirectory(app *redskyappsv1alpha1.Application) string {
	if path := app.Annotations[kioutil.PathAnnotation]; path != "" {
		return filepath.Dir(path)
	}
	return ""
}

// LoadResources loads all of the resources for an application, using the supplied file system
// to load file based resources (if necessary).
func LoadResources(app *redskyappsv1alpha1.Application, _ filesys.FileSystem) (resmap.ResMap, error) {
	kf := scan.NewKonjureFilter(WorkingDirectory(app), nil)
	kf.KeepStatus = false

	var buf bytes.Buffer
	err := kio.Pipeline{
		Inputs:  []kio.Reader{app.Resources},
		Filters: []kio.Filter{kf},
		Outputs: []kio.Writer{&kio.ByteWriter{Writer: &buf}},
	}.Execute()
	if err != nil {
		return nil, err
	}

	dp := provider.NewDefaultDepProvider()
	f := resmap.NewFactory(dp.GetResourceFactory(), dp.GetConflictDetectorFactory())
	return f.NewResMapFromBytes(buf.Bytes())
}

func cleanName(n string) string {
	return strings.Map(func(r rune) rune {
		r = unicode.ToLower(r)
		if r >= 'a' && r <= 'z' {
			return r
		}
		if r >= '0' && r <= '9' {
			return r
		}
		if r == '-' || r == '.' {
			return r
		}
		return -1
	}, n)
}

type experimentNameLexer struct {
	input     string
	pos       int
	tokenType int
	tokens    [][]string
}

var errEos = errors.New("eos")

func newLexer(application *redskyappsv1alpha1.Application, name string) *experimentNameLexer {
	// Build the token library from the defaulted application
	app := application.DeepCopy()
	app.Default()

	e := &experimentNameLexer{input: name, tokens: make([][]string, 3)}
	e.tokens[0] = []string{app.Name}
	for _, s := range app.Scenarios {
		e.tokens[1] = append(e.tokens[1], cleanName(s.Name))
	}
	for _, o := range app.Objectives {
		e.tokens[2] = append(e.tokens[2], cleanName(o.Name))
	}
	return e
}

func (e *experimentNameLexer) next() (string, error) {
	// End-of-stream
	if e.pos == len(e.input) {
		return "", errEos
	}

	// Find all the matches
	match := make([]string, 0, len(e.tokens[e.tokenType]))
	for _, t := range e.tokens[e.tokenType] {
		if e.pos+len(t) <= len(e.input) && e.input[e.pos:e.pos+len(t)] == t {
			match = append(match, t)
		}
	}

	switch len(match) {
	case 0:
		return "", fmt.Errorf("invalid name '%s', could not find %s", e.input, strings.Join(e.tokens[e.tokenType], ", "))
	case 1:
		return e.consume(match[0]), nil
	default:
		// There were multiple matches check to see if any of them allow us to parse to the end
		matchIndex := -1
		for i, m := range match {
			ee := *e // Copy
			ee.consume(m)
			_, err := ee.next()
			for err == nil {
				_, err = ee.next()
			}
			if err == errEos {
				// We consumed the match and still got to the end, this match was good
				if matchIndex >= 0 {
					return "", &AmbiguousNameError{Name: e.input}
				}
				matchIndex = i
			}
		}
		if matchIndex < 0 {
			return "", fmt.Errorf("invalid name '%s'", e.input)
		}
		return e.consume(match[matchIndex]), nil
	}
}

func (e *experimentNameLexer) consume(s string) string {
	e.pos += len(s)
	if e.pos < len(e.input) && e.input[e.pos] == '-' {
		e.pos++
	}
	if e.tokenType+1 < len(e.tokens) {
		e.tokenType++
	}
	return s
}
