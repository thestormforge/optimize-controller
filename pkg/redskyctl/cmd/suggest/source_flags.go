package suggest

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// The captured flag values can be used to directly to suggest assignments
var _ SuggestionSource = &SuggestionSourceFlags{}

type SuggestionSourceFlags struct {
	Assignments map[string]string
}

func NewSuggestionSourceFlags() *SuggestionSourceFlags {
	return &SuggestionSourceFlags{}
}

func (f *SuggestionSourceFlags) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringToStringVarP(&f.Assignments, "assign", "A", nil, "Assign an explicit value to a parameter")
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
	return 0, fmt.Errorf("no assignment for parameter: %s", name)
}

func (f *SuggestionSourceFlags) AssignDouble(name string, min, max float64) (float64, error) {
	if a, ok := f.Assignments[name]; ok {
		f, err := strconv.ParseFloat(a, 64)
		if err == nil && (f < min || f > max) {
			return f, fmt.Errorf("assignment out of bounds: %s=%f (expected [%f,%f])", name, f, min, max)
		}
		return f, err
	}
	return 0, fmt.Errorf("no assignment for parameter: %s", name)
}
