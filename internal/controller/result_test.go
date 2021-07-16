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

package controller

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thestormforge/optimize-go/pkg/api"
	experimentsv1alpha1 "github.com/thestormforge/optimize-go/pkg/api/experiments/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type testFrameStack struct {
	files []string
	idx   int
}

func (tfs *testFrameStack) Next() (runtime.Frame, bool) {
	defer func() {
		tfs.idx++
	}()

	more := len(tfs.files) > (tfs.idx + 1)

	return runtime.Frame{
		File: tfs.files[tfs.idx],
	}, more
}

func TestGuessController(t *testing.T) {
	cases := []struct {
		desc string
		name string
		fs   frameStack
	}{
		{
			desc: "no controllers",
			name: "controller",
			fs: &testFrameStack{
				files: []string{
					"/a/b/c.go",
					"/e/f/g.go",
					"/h/i/j.go",
				},
			},
		},
		{
			desc: "controllers",
			name: "111",
			fs: &testFrameStack{
				files: []string{
					"/a/b/c.go",
					"/e/f/g.go",
					"/h/controllers/111_controller.go",
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			name := guessController(c.fs)
			assert.Equal(t, c.name, name)
		})
	}
}

func TestGetFrames(t *testing.T) {
	f, more := getFrames().Next()
	assert.True(t, more)
	assert.Equal(t, "testing.tRunner", f.Function)
}

func TestRequeueConflictForFrameStack(t *testing.T) {
	cases := []struct {
		desc        string
		err         error
		fs          frameStack
		result      *ctrl.Result
		expectedErr error
	}{
		{
			desc:        "not conflict",
			err:         fmt.Errorf("111"),
			result:      &ctrl.Result{},
			expectedErr: fmt.Errorf("111"),
		},
		{
			desc: "conflict",
			err: &errors.StatusError{
				ErrStatus: metav1.Status{
					Reason: metav1.StatusReasonConflict,
				},
			},
			fs: &testFrameStack{
				files: []string{
					"/a/b/c.go",
				},
			},
			result: &ctrl.Result{
				Requeue: true,
			},
			expectedErr: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			result, err := requeueConflictForFrameStack(c.err, c.fs)
			assert.Equal(t, c.result, result)
			assert.Equal(t, c.expectedErr, err)
		})
	}
}

func TestRequeueIfUnavailable(t *testing.T) {
	cases := []struct {
		desc        string
		err         error
		fs          frameStack
		result      *ctrl.Result
		expectedErr error
	}{
		{
			desc:        "not correct error type",
			err:         fmt.Errorf("111"),
			result:      &ctrl.Result{},
			expectedErr: fmt.Errorf("111"),
		},
		{
			desc: "trial unavailable",
			err: &api.Error{
				Type:       experimentsv1alpha1.ErrTrialUnavailable,
				RetryAfter: 111,
			},
			result: &ctrl.Result{
				RequeueAfter: 5 * time.Second, // 5 second minimum
			},
			expectedErr: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			result, err := RequeueIfUnavailable(c.err)
			assert.Equal(t, c.result, result)
			assert.Equal(t, c.expectedErr, err)
		})
	}
}
