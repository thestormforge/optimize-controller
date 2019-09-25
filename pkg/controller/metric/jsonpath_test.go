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

package metric

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/client-go/util/jsonpath"
)

func TestJsonPath(t *testing.T) {
	g := NewGomegaWithT(t)
	name := "testing"
	query := "{.foo.bar[0].gus}"
	body := bytes.NewBufferString(`{
"foo" : {
    "bar" : [ {"gus" : 1 }, { "baz" : 2.2 } ]
  }
}`)

	// Unmarshal as generic JSON
	data := make(map[string]interface{})
	err := json.NewDecoder(body).Decode(&data)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Evaluate the JSON path
	jp := jsonpath.New(name)
	if err := jp.Parse(query); err != nil {
		t.Fail()
		return
	}
	values, err := jp.FindResults(data)
	if err != nil {
		t.Fail()
		return
	}

	var value float64
	if len(values) == 1 && len(values[0]) == 1 {
		v := reflect.ValueOf(values[0][0].Interface())
		switch v.Kind() {
		case reflect.Float64:
			value = v.Float()
		case reflect.String:
			if v, err := strconv.ParseFloat(v.String(), 64); err != nil {
				t.Fail()
				return
			} else {
				value = v
			}
		default:
			t.Fail()
			return
		}
	}

	g.Expect(value).Should(Equal(float64(1.0)))
}
