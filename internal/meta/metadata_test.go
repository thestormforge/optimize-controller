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

package meta

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testObj struct {
	metav1.ObjectMeta
}

func TestAddFinalizer(t *testing.T) {
	cases := []struct {
		desc           string
		obj            metav1.Object
		finalizer      string
		finalizerAdded bool
	}{
		{
			desc: "zero deletion timestamp",
			obj: &testObj{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &metav1.Time{
						Time: time.Now(),
					},
				},
			},
			finalizerAdded: false,
		},
		{
			desc: "finalizer already added",
			obj: &testObj{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"111"},
				},
			},
			finalizer:      "111",
			finalizerAdded: false,
		},
		{
			desc: "set finalizer",
			obj: &testObj{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{},
				},
			},
			finalizer:      "111",
			finalizerAdded: true,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			finalizerAdded := AddFinalizer(c.obj, c.finalizer)
			assert.Equal(t, c.finalizerAdded, finalizerAdded)
			if finalizerAdded {
				assert.Contains(t, c.obj.GetFinalizers(), c.finalizer)
			}
		})
	}
}

func TestRemoveFinalizer(t *testing.T) {
	cases := []struct {
		desc             string
		obj              metav1.Object
		finalizer        string
		finalizerRemoved bool
	}{
		{
			desc:             "no finalizers",
			obj:              &testObj{},
			finalizerRemoved: false,
		},
		{
			desc: "finalizer not found",
			obj: &testObj{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"111"},
				},
			},
			finalizer:        "222",
			finalizerRemoved: false,
		},
		{
			desc: "remove finalizer",
			obj: &testObj{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"111", "222", "333"},
				},
			},
			finalizer:        "111",
			finalizerRemoved: true,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			finalizerRemoved := RemoveFinalizer(c.obj, c.finalizer)
			assert.Equal(t, c.finalizerRemoved, finalizerRemoved)
			if finalizerRemoved {
				assert.NotContains(t, c.obj.GetFinalizers(), c.finalizer)
			}
		})
	}
}

func TestHasFinalizer(t *testing.T) {
	cases := []struct {
		desc         string
		obj          metav1.Object
		finalizer    string
		hasFinalizer bool
	}{
		{
			desc: "does not have finalizer",
			obj: &testObj{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"111"},
				},
			},
			finalizer:    "222",
			hasFinalizer: false,
		},
		{
			desc: "has finalizer",
			obj: &testObj{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"111", "222", "333"},
				},
			},
			finalizer:    "111",
			hasFinalizer: true,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			hasFinalizer := HasFinalizer(c.obj, c.finalizer)
			assert.Equal(t, c.hasFinalizer, hasFinalizer)
		})
	}
}

func TestAddLabel(t *testing.T) {
	cases := []struct {
		desc           string
		obj            metav1.Object
		label          string
		value          string
		expectedLables map[string]string
	}{
		{
			desc:           "nil lables",
			obj:            &testObj{},
			label:          "111",
			value:          "222",
			expectedLables: map[string]string{"111": "222"},
		},
		{
			desc: "existing lables",
			obj: &testObj{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"_": "__"},
				},
			},
			label:          "111",
			value:          "222",
			expectedLables: map[string]string{"_": "__", "111": "222"},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			AddLabel(c.obj, c.label, c.value)
			assert.EqualValues(t, c.obj.GetLabels(), c.expectedLables)
		})
	}
}
