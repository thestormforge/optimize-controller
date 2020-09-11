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

package v1alpha1

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1/numstr"
)

// LowerBound attempts to return the lower bound for this parameter.
func (p *Parameter) LowerBound() (*numstr.NumberOrString, error) {
	if p.Type == ParameterTypeCategorical {
		if len(p.Values) == 0 {
			return nil, fmt.Errorf("unable to determine categorical minimum bound")
		}
		return &numstr.NumberOrString{StrVal: p.Values[0], IsString: true}, nil
	}

	if p.Bounds == nil {
		return nil, fmt.Errorf("unable to determine numeric minimum bound")
	}

	return &numstr.NumberOrString{NumVal: p.Bounds.Min}, nil
}

// UpperBound attempts to return the upper bound for this parameter.
func (p *Parameter) UpperBound() (*numstr.NumberOrString, error) {
	if p.Type == ParameterTypeCategorical {
		if len(p.Values) == 0 {
			return nil, fmt.Errorf("unable to determine categorical maximum bound")
		}
		return &numstr.NumberOrString{StrVal: p.Values[len(p.Values)-1], IsString: true}, nil
	}

	if p.Bounds == nil {
		return nil, fmt.Errorf("unable to determine numeric maximum bound")
	}

	return &numstr.NumberOrString{NumVal: p.Bounds.Max}, nil
}

// ParseValue attempts to parse the supplied value into a NumberOrString based on the type of this parameter.
func (p *Parameter) ParseValue(s string) (*numstr.NumberOrString, error) {
	var v numstr.NumberOrString
	switch p.Type {
	case ParameterTypeInteger:
		if _, err := strconv.ParseInt(s, 10, 64); err != nil {
			return nil, err
		}
		v = numstr.FromNumber(json.Number(s))
	case ParameterTypeDouble:
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return nil, err
		}
		v = numstr.FromNumber(json.Number(s))
	case ParameterTypeCategorical:
		v = numstr.FromString(s)
	}
	return &v, nil
}

// CheckParameterValue validates that the supplied value can be used for a parameter.
func CheckParameterValue(p *Parameter, v *numstr.NumberOrString) error {
	if p.Type == ParameterTypeCategorical {
		if !v.IsString {
			return fmt.Errorf("categorical value must be a string: %s", v.String())
		}
		for _, allowed := range p.Values {
			if v.StrVal == allowed {
				return nil
			}
		}
		return fmt.Errorf("categorical value is out of range: %s [%s]", v.String(), strings.Join(p.Values, ", "))
	}

	if v.IsString {
		return fmt.Errorf("numeric value must not be a string: %s", v.String())
	}

	lower, err := p.LowerBound()
	if err != nil {
		return err
	}
	upper, err := p.UpperBound()
	if err != nil {
		return err
	}

	switch p.Type {
	case ParameterTypeInteger:
		val := v.Int64Value()
		min, max := lower.Int64Value(), upper.Int64Value()
		if val < min || val > max {
			return fmt.Errorf("integer value is out of range [%d-%d]: %d", min, max, val)
		}
	case ParameterTypeDouble:
		val := v.Float64Value()
		min, max := lower.Float64Value(), upper.Float64Value()
		if val < min || val > max {
			return fmt.Errorf("double value is out of range [%f-%f]: %f", min, max, val)
		}
	default:
		return fmt.Errorf("unknown parameter type: %s", p.Type)
	}
	return nil
}
