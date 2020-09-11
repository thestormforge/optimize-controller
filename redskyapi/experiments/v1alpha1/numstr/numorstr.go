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

package numstr

import (
	"encoding/json"
	"strconv"
)

// NumberOrString is value that can a JSON number or string.
type NumberOrString struct {
	IsString bool
	NumVal   json.Number
	StrVal   string
}

// FromInt64 returns the supplied value as a NumberOrString
func FromInt64(val int64) NumberOrString {
	return NumberOrString{NumVal: json.Number(strconv.FormatInt(val, 10))}
}

// FromFloat64 returns the supplied value as a NumberOrString
func FromFloat64(val float64) NumberOrString {
	return NumberOrString{NumVal: json.Number(strconv.FormatFloat(val, 'f', -1, 64))}
}

// FromNumber returns the supplied value as a NumberOrString
func FromNumber(val json.Number) NumberOrString {
	return NumberOrString{NumVal: val}
}

// FromString returns the supplied value as a NumberOrString
func FromString(val string) NumberOrString {
	return NumberOrString{StrVal: val, IsString: true}
}

// String coerces the value to a string.
func (s *NumberOrString) String() string {
	if s.IsString {
		return s.StrVal
	}
	return s.NumVal.String()
}

// Int64Value coerces the value to an int64.
func (s *NumberOrString) Int64Value() int64 {
	if s.IsString {
		v, _ := strconv.ParseInt(s.StrVal, 10, 64)
		return v
	}
	v, _ := s.NumVal.Int64()
	return v
}

// Float64Value coerces the value to a float64.
func (s *NumberOrString) Float64Value() float64 {
	if s.IsString {
		v, _ := strconv.ParseFloat(s.StrVal, 64)
		return v
	}
	v, _ := s.NumVal.Float64()
	return v
}

// MarshalJSON writes the value with the appropriate type.
func (s NumberOrString) MarshalJSON() ([]byte, error) {
	if s.IsString {
		return json.Marshal(s.StrVal)
	}
	return json.Marshal(s.NumVal)
}

// UnmarshalJSON reads the value from either a string or number.
func (s *NumberOrString) UnmarshalJSON(b []byte) error {
	if b[0] == '"' {
		s.IsString = true
		return json.Unmarshal(b, &s.StrVal)
	}
	return json.Unmarshal(b, &s.NumVal)
}
