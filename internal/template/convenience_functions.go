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

package template

import (
	"fmt"
	"math"
)

const (
	iByte = 1024
	aByte = 1000
)

// Giga
func gb(query string) string {
	return fmt.Sprintf("%s/%.f", query, math.Pow(aByte, 3))
}
func mb(query string) string {
	return fmt.Sprintf("%s/%.f", query, math.Pow(aByte, 2))
}
func kb(query string) string {
	return fmt.Sprintf("%s/%.f", query, math.Pow(aByte, 1))
}

// Gibi
func gib(query string) string {
	return fmt.Sprintf("%s/%.f", query, math.Pow(iByte, 3))
}
func mib(query string) string {
	return fmt.Sprintf("%s/%.f", query, math.Pow(iByte, 2))
}
func kib(query string) string {
	return fmt.Sprintf("%s/%.f", query, math.Pow(iByte, 1))
}
