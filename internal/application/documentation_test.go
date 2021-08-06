/*
Copyright 2021 GramLabs, Inc.

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
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
)

func TestJSONFields(t *testing.T) {
	// Index the top-level JSON field names on the Application type
	fieldNames := make(map[string]bool)
	appType := reflect.TypeOf(optimizeappsv1alpha1.Application{})
	for i := 0; i < appType.NumField(); i++ {
		fieldNames[strings.Split(appType.Field(i).Tag.Get("json"), ",")[0]] = true
	}

	// Check all the headComments keys
	t.Run("headComments", func(t *testing.T) {
		for k := range headComments {
			assert.True(t, fieldNames[k], "invalid key %q", k)
		}
	})

	// Check all the footComments keys
	t.Run("footComments", func(t *testing.T) {
		for k := range footComments {
			assert.True(t, fieldNames[k], "invalid key %q", k)
		}
	})

	// Required contains field names in both the key and value so check both
	t.Run("required", func(t *testing.T) {
		for k, v := range required {
			assert.True(t, fieldNames[k] || k == "", "invalid key %q", k)
			assert.True(t, fieldNames[v] || v == "", "invalid value %q", v)
		}
	})
}
