package suggest

import (
	"bufio"
	"fmt"
	"strconv"

	"github.com/gramLabs/redsky/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

// The captured flag values can be used to directly to suggest assignments
var _ SuggestionSource = &SuggestionSourceFlags{}

type SuggestionSourceFlags struct {
	Assignments      map[string]string
	AllowInteractive bool

	util.IOStreams
}

func NewSuggestionSourceFlags(ioStreams util.IOStreams) *SuggestionSourceFlags {
	return &SuggestionSourceFlags{IOStreams: ioStreams}
}

func (f *SuggestionSourceFlags) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringToStringVarP(&f.Assignments, "assign", "A", nil, "Assign an explicit value to a parameter")
	cmd.Flags().BoolVar(&f.AllowInteractive, "interactive", false, "Allow interactive prompts for unspecified parameter assignments")
	// TODO Do we want to define a default behavior for when an assignment is not found? e.g. "use random"
}

func (f *SuggestionSourceFlags) AssignInt(name string, min, max int64) (int64, error) {
	if a, ok := f.Assignments[name]; ok {
		i, err := strconv.ParseInt(a, 10, 64)
		if err == nil && (i < min || i > max) {
			return i, fmt.Errorf("assignment out of bounds: %s=%d (expected [%d,%d])", name, i, min, max)
		}
		return i, err
	}

	if f.AllowInteractive {
		scanner := bufio.NewScanner(f.In)
		if _, err := fmt.Fprintf(f.Out, "Assignment for integer parameter '%s' [%d,%d]: ", name, min, max); err != nil {
			return 0, err
		}
		for attempts := 1; attempts < 3 && scanner.Scan(); attempts++ {
			i, err := strconv.ParseInt(scanner.Text(), 10, 64)
			if err != nil || (i < min || i > max) {
				_, _ = fmt.Fprintf(f.Out, "Invalid assignment, try again: ")
				continue
			}
			return i, err
		}
		if err := scanner.Err(); err != nil {
			return 0, err
		}
	}

	return 0, fmt.Errorf("no assignment for parameter: %s", name)
}

func (f *SuggestionSourceFlags) AssignDouble(name string, min, max float64) (float64, error) {
	if a, ok := f.Assignments[name]; ok {
		d, err := strconv.ParseFloat(a, 64)
		if err == nil && (d < min || d > max) {
			return d, fmt.Errorf("assignment out of bounds: %s=%f (expected [%f,%f])", name, d, min, max)
		}
		return d, err
	}

	if f.AllowInteractive {
		scanner := bufio.NewScanner(f.In)
		if _, err := fmt.Fprintf(f.Out, "Assignment for double parameter '%s' [%f,%f]: ", name, min, max); err != nil {
			return 0, err
		}
		for attempts := 1; attempts < 3 && scanner.Scan(); attempts++ {
			d, err := strconv.ParseFloat(scanner.Text(), 64)
			if err != nil || (d < min || d > max) {
				_, _ = fmt.Fprintf(f.Out, "Invalid assignment, try again: ")
				continue
			}
			return d, err
		}
		if err := scanner.Err(); err != nil {
			return 0, err
		}
	}

	return 0, fmt.Errorf("no assignment for parameter: %s", name)
}
