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

package suggest

import (
	"bufio"
	"fmt"
	"math/rand"
	"strconv"

	"github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

// The captured flag values can be used to directly to suggest assignments
var _ SuggestionSource = &SuggestionSourceFlags{}

type SuggestionSourceFlags struct {
	Assignments      map[string]string
	AllowInteractive bool
	DefaultBehavior  string

	util.IOStreams
}

func NewSuggestionSourceFlags(ioStreams util.IOStreams) *SuggestionSourceFlags {
	return &SuggestionSourceFlags{IOStreams: ioStreams}
}

func (f *SuggestionSourceFlags) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringToStringVarP(&f.Assignments, "assign", "A", nil, "Assign an explicit value to a parameter.")
	cmd.Flags().BoolVar(&f.AllowInteractive, "interactive", false, "Allow interactive prompts for unspecified parameter assignments.")
	cmd.Flags().StringVar(&f.DefaultBehavior, "default", "", "Select the behavior for default values; one of: none|min|max|rand.")
}

func (f *SuggestionSourceFlags) AssignInt(name string, min, max int64, def *int64) (int64, error) {
	if a, ok := f.Assignments[name]; ok {
		i, err := strconv.ParseInt(a, 10, 64)
		if err == nil && (i < min || i > max) {
			return i, fmt.Errorf("assignment out of bounds: %s=%d (expected [%d,%d])", name, i, min, max)
		}
		return i, err
	}

	def = f.defaultInt(min, max, def)

	if f.AllowInteractive {
		return f.assignIntInteractive(name, min, max, def)
	}

	if def != nil {
		return *def, nil
	}

	return 0, fmt.Errorf("no assignment for parameter: %s", name)
}

func (f *SuggestionSourceFlags) AssignDouble(name string, min, max float64, def *float64) (float64, error) {
	if a, ok := f.Assignments[name]; ok {
		d, err := strconv.ParseFloat(a, 64)
		if err == nil && (d < min || d > max) {
			return d, fmt.Errorf("assignment out of bounds: %s=%f (expected [%f,%f])", name, d, min, max)
		}
		return d, err
	}

	def = f.defaultDouble(min, max, def)

	if f.AllowInteractive {
		return f.assignDoubleInteractive(name, min, max, def)
	}

	if def != nil {
		return *def, nil
	}

	return 0, fmt.Errorf("no assignment for parameter: %s", name)
}

func (f *SuggestionSourceFlags) assignIntInteractive(name string, min, max int64, def *int64) (int64, error) {
	if def != nil {
		_, _ = fmt.Fprintf(f.Out, "Assignment for integer parameter '%s' [%d,%d] (%d): ", name, min, max, *def)
	} else {
		_, _ = fmt.Fprintf(f.Out, "Assignment for integer parameter '%s' [%d,%d]: ", name, min, max)
	}

	s := bufio.NewScanner(f.In)
	for attempts := 0; attempts < 3; attempts++ {
		if attempts > 0 {
			_, _ = fmt.Fprintf(f.Out, "Invalid assignment, try again: ")
		}
		if !s.Scan() {
			break
		}
		text := s.Text()
		if text == "" && def != nil {
			return *def, nil
		}
		i, err := strconv.ParseInt(text, 10, 64)
		if err != nil || (i < min || i > max) {
			continue
		}
		return i, err
	}

	if err := s.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("no assignment for parameter: %s", name)
}

func (f *SuggestionSourceFlags) assignDoubleInteractive(name string, min, max float64, def *float64) (float64, error) {
	if def != nil {
		_, _ = fmt.Fprintf(f.Out, "Assignment for double parameter '%s' [%f,%f] (%f): ", name, min, max, *def)
	} else {
		_, _ = fmt.Fprintf(f.Out, "Assignment for double parameter '%s' [%f,%f]: ", name, min, max)
	}

	s := bufio.NewScanner(f.In)
	for attempts := 0; attempts < 3; attempts++ {
		if attempts > 0 {
			_, _ = fmt.Fprintf(f.Out, "Invalid assignment, try again: ")
		}
		if !s.Scan() {
			break
		}
		text := s.Text()
		if text == "" && def != nil {
			return *def, nil
		}
		d, err := strconv.ParseFloat(text, 64)
		if err != nil || (d < min || d > max) {
			continue
		}
		return d, err
	}

	if err := s.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("no assignment for parameter: %s", name)
}

func (f *SuggestionSourceFlags) defaultInt(min, max int64, def *int64) *int64 {
	switch f.DefaultBehavior {
	case "none":
		return nil
	case "min":
		return &min
	case "max":
		return &max
	case "rand":
		d := rand.Int63n(max-min) + min
		return &d
	default:
		return def
	}
}

func (f *SuggestionSourceFlags) defaultDouble(min, max float64, def *float64) *float64 {
	switch f.DefaultBehavior {
	case "none":
		return nil
	case "min":
		return &min
	case "max":
		return &max
	case "rand":
		d := rand.Float64()*max + min
		return &d
	default:
		return def
	}
}
