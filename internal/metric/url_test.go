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

package metric

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestURL_String(t *testing.T) {
	cases := []struct {
		desc     string
		url      *URL
		expected string
	}{
		{
			desc: "empty",
			url:  &URL{},
		},
		{
			desc: "scheme",
			url: &URL{
				Scheme: "https",
			},
			expected: "https:",
		},
		{
			desc: "host",
			url: &URL{
				Host: "example.com",
			},
			expected: "http://example.com",
		},
		{
			desc: "host and int port",
			url: &URL{
				Host: "example.com",
				Port: intstr.FromInt(80),
			},
			expected: "http://example.com:80",
		},
		{
			desc: "host and string port",
			url: &URL{
				Host: "example.com",
				Port: intstr.FromString("http"),
			},
			expected: "http://example.com:http",
		},
		{
			desc: "query only",
			url: &URL{
				RawQuery: "foo=bar",
			},
			expected: "?foo=bar",
		},
		{
			desc: "label selector",
			url: &URL{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar", "test": "true"},
				},
			},
			expected: "http://{foo=bar,test=true}",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			assert.Equal(t, c.expected, c.url.String())
		})
	}
}

func TestParseURL(t *testing.T) {
	cases := []struct {
		desc     string
		input    string
		expected URL
		err      error
	}{
		{
			desc: "empty",
		},
		{
			desc:  "scheme",
			input: "https:",
			expected: URL{
				Scheme: "https",
			},
		},
		{
			desc:  "scheme and host",
			input: "http://example.com",
			expected: URL{
				Scheme: "http",
				Host:   "example.com",
			},
		},
		{
			desc:  "scheme host port",
			input: "http://example.com:8080",
			expected: URL{
				Scheme: "http",
				Host:   "example.com",
				Port:   intstr.FromInt(8080),
			},
		},
		{
			desc:  "scheme host port path",
			input: "http://example.com:8080/",
			expected: URL{
				Scheme: "http",
				Host:   "example.com",
				Port:   intstr.FromInt(8080),
				Path:   "/",
			},
		},
		{
			desc:  "scheme host port path query",
			input: "http://example.com:8080/foo?bar=true",
			expected: URL{
				Scheme:   "http",
				Host:     "example.com",
				Port:     intstr.FromInt(8080),
				Path:     "/foo",
				RawQuery: "bar=true",
			},
		},
		{
			desc:  "named port",
			input: "http://example.com:http",
			expected: URL{
				Scheme: "http",
				Host:   "example.com",
				Port:   intstr.FromString("http"),
			},
		},
		{
			desc:  "label selector",
			input: "http://{foo=bar}",
			expected: URL{
				Scheme: "http",
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
			},
		},
		{
			desc:  "prefixed label selector",
			input: "http://{test/foo=bar}",
			expected: URL{
				Scheme: "http",
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"test/foo": "bar"},
				},
			},
		},
		{
			desc:  "query only",
			input: "?foo=bar",
			expected: URL{
				RawQuery: "foo=bar",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			actual, err := ParseURL(c.input)
			if c.err != nil {
				assert.EqualError(t, err, c.err.Error())
			} else if assert.NoError(t, err) {
				assert.Equal(t, c.expected, actual)
			}
		})
	}
}

func TestURL_ForceSelector(t *testing.T) {
	newURL := &URL{
		Scheme: "http",
		Host:   "example.com",
	}
	legacyURL := &URL{
		Scheme:   newURL.Scheme,
		Selector: newURL.ForceSelector(),
	}

	assert.Equal(t, "http://example.com", newURL.String())
	assert.Equal(t, "http://{internal.redskyops.dev/host=example.com}", legacyURL.String())
	actual, err := ParseURL(legacyURL.String())
	if assert.NoError(t, err) {
		assert.Equal(t, newURL, &actual)
	}
}

func TestBracketAwareSplit(t *testing.T) {
	cases := []struct {
		desc  string
		input string
		sep   byte
		cutc  bool
		left  string
		right string
	}{
		{
			desc:  "zero",
			input: "{foo}/test", sep: '/', cutc: false,
			left: "{foo}", right: "/test",
		},
		{
			desc:  "one",
			input: "{foo/bar}/test", sep: '/', cutc: false,
			left: "{foo/bar}", right: "/test",
		},
		{
			desc:  "two",
			input: "{foo/bar/gus}/test", sep: '/', cutc: false,
			left: "{foo/bar/gus}", right: "/test",
		},
		{
			desc:  "no bracket",
			input: "foo/test", sep: '/', cutc: false,
			left: "foo", right: "/test",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			left, right := bracketAwareSplit(c.input, c.sep, c.cutc)
			assert.Equal(t, c.left, left)
			assert.Equal(t, c.right, right)
		})
	}
}

func TestServiceListResolver_LookupAuthority(t *testing.T) {
	cases := []struct {
		desc        string
		services    *corev1.ServiceList
		host        string
		port        intstr.IntOrString
		authorities []string
		err         error
	}{
		{
			desc: "empty",
		},
		{
			desc:        "host",
			host:        "example.com",
			authorities: []string{"example.com"},
		},
		{
			desc: "single service",
			services: &corev1.ServiceList{
				Items: []corev1.Service{{Spec: corev1.ServiceSpec{
					ClusterIP: "1.1.1.1",
				}}},
			},
			authorities: []string{"1.1.1.1"},
		},
		{
			desc: "default zero port",
			services: &corev1.ServiceList{
				Items: []corev1.Service{{Spec: corev1.ServiceSpec{
					ClusterIP: "1.1.1.1",
					Ports:     []corev1.ServicePort{{Name: "anything", Port: 8080}},
				}}},
			},
			authorities: []string{"1.1.1.1:8080"},
		},
		{
			desc: "default empty port",
			services: &corev1.ServiceList{
				Items: []corev1.Service{{Spec: corev1.ServiceSpec{
					ClusterIP: "1.1.1.1",
					Ports:     []corev1.ServicePort{{Name: "anything", Port: 8080}},
				}}},
			},
			port:        intstr.FromString(""),
			authorities: []string{"1.1.1.1:8080"},
		},
		{
			desc: "named port",
			services: &corev1.ServiceList{
				Items: []corev1.Service{{Spec: corev1.ServiceSpec{
					ClusterIP: "1.1.1.1",
					Ports:     []corev1.ServicePort{{Name: "test", Port: 8080}, {Name: "notused", Port: 8888}},
				}}},
			},
			port:        intstr.FromString("test"),
			authorities: []string{"1.1.1.1:8080"},
		},
		{
			desc: "multiple services different named ports",
			services: &corev1.ServiceList{
				Items: []corev1.Service{
					{Spec: corev1.ServiceSpec{
						ClusterIP: "1.1.1.1",
						Ports:     []corev1.ServicePort{{Name: "test", Port: 1111}},
					}},
					{Spec: corev1.ServiceSpec{
						ClusterIP: "2.2.2.2",
						Ports:     []corev1.ServicePort{{Name: "test", Port: 2222}},
					}},
				},
			},
			port:        intstr.FromString(""),
			authorities: []string{"1.1.1.1:1111", "2.2.2.2:2222"},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			r := &ServiceListResolver{Services: c.services}
			authorities, err := r.LookupAuthority("http", c.host, c.port)
			if c.err != nil {
				assert.EqualError(t, err, c.err.Error())
			} else if assert.NoError(t, err) {
				assert.Equal(t, c.authorities, authorities)
			}
		})
	}
}
