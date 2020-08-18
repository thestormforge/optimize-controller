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
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// URL is a metric source URL; it is not a proper URL in that it allows a non-standard authority section and no fragment.
// The metric source URL authority can be a standard authority section. It also allows the port specification to be a
// service name (e.g. `http://example.com:http` is the same as `http://example.com:80` or just `http://example.com`).
// Additionally, the host name itself can be replaced with a Kubernetes label selector expression wrapped in curley
// brackets (e.g. `http://{environment=production,tier=frontend}:http`), using this syntax the metric source URL is
// resolved against all matching services.
// Metric source URLs _must_ be absolute URLs.
type URL struct {
	Scheme   string
	Selector *metav1.LabelSelector
	Host     string
	Port     intstr.IntOrString
	Path     string
	RawQuery string
}

// newURL is for internal use, it creates a real URL from the current values (defaulting the scheme to "http")
func (u *URL) newURL() *url.URL {
	uu := &url.URL{
		Scheme:   strings.ToLower(u.Scheme),
		Host:     u.Host,
		Path:     u.Path,
		RawQuery: u.RawQuery,
	}
	if uu.Scheme == "" {
		// The choice of "http" as a default comes from older code
		uu.Scheme = "http"
	}
	return uu
}

// Query returns the parsed query parameters of this URL
func (u *URL) Query() url.Values {
	v, _ := url.ParseQuery(u.RawQuery)
	return v
}

// ForceSelector is used force a metrics URL into a legacy format where explicit host names were not allowed
func (u *URL) ForceSelector() *metav1.LabelSelector {
	if u.Selector != nil {
		return u.Selector
	}
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{"internal.redskyops.dev/host": u.Host},
	}
}

// String returns a reversible string representation of this URL.
// IMPORTANT: a metric URL IS NOT a valid URL, the authority section is not standard to represent a service selector or named port.
func (u *URL) String() string {
	// We can use a manually constructed URL object for formatting
	uu := u.newURL()

	// If we only have a path and/or query, use the standard string construction
	if u.Scheme == "" && u.Host == "" && u.Port.StrVal == "" && u.Port.IntVal == 0 && u.Selector == nil {
		uu.Scheme = ""
		return uu.String()
	}

	// Make the port easier to append
	port := u.Port.String()
	if port == "0" {
		port = ""
	} else if port != "" {
		port = ":" + port
	}

	// If there is a host, just ignore the selector and use the standard string construction
	// NOTE: the url.URL does not check the syntax of the Host field when constructed manually so the string port is OK
	if uu.Host != "" {
		uu.Host += port
		return uu.String()
	}

	// Write a bogus authority
	var buf strings.Builder
	buf.WriteString(uu.Scheme)
	buf.WriteString("://{")
	if sel := metav1.FormatLabelSelector(u.Selector); sel != "<none>" && sel != "<error>" {
		buf.WriteString(sel)
	}
	buf.WriteByte('}')
	buf.WriteString(port)

	// Use the standard URL for the rest of the string
	uu.Scheme = ""
	uu.Host = ""
	buf.WriteString(uu.String())
	return buf.String()
}

// ParseURL parses the supplied string into a URL value
func ParseURL(str string) (URL, error) {
	u := URL{}

	// Extract the scheme
	u.Scheme, str = split(str, ':', true)
	if str == "" && strings.HasPrefix(u.Scheme, "?") {
		// Special case for a URL that only has query parameters
		str = u.Scheme
		u.Scheme = ""
	}
	u.Scheme = strings.ToLower(u.Scheme)

	// Extract the query
	str, u.RawQuery = split(str, '?', true)

	// Extract the path (labels inside the {} may contain slashes)
	str = strings.TrimLeft(str, "/")
	str, u.Path = bracketAwareSplit(str, '/', false)

	// Parse the authority
	u.Host, str = split(str, ':', true)
	if strings.HasPrefix(u.Host, "{") && strings.HasSuffix(u.Host, "}") {
		var err error
		u.Selector, err = metav1.ParseToLabelSelector(strings.Trim(u.Host, "{}"))
		if err != nil {
			return URL{}, err
		}
		u.Host = u.Selector.MatchLabels["internal.redskyops.dev/host"]
		delete(u.Selector.MatchLabels, "internal.redskyops.dev/host")
		if len(u.Selector.MatchExpressions) == 0 {
			if len(u.Selector.MatchLabels) == 0 {
				u.Selector = nil
			} else {
				u.Selector.MatchExpressions = nil
			}
		}
	}
	if str != "" {
		u.Port = intstr.Parse(str)
	}

	return u, nil
}

// Resolver is used to lookup a URL authority based on an optional host name and named (or numeric) port
type Resolver interface {
	// LookupAuthority resolves the optional host name and named or numeric port to at least one URL authority ("host") value
	LookupAuthority(scheme, host string, port intstr.IntOrString) ([]string, error)
}

// NewResolver returns a metric URL resolver based on the supplied target object
func NewResolver(target runtime.Object) Resolver {
	// TODO We can probably accept more types here, single services, pod, list of pods, etc.
	// TODO Or we can go the other way and get rid of this all together and just use standard URLs
	switch t := target.(type) {
	case *corev1.ServiceList:
		return &ServiceListResolver{Services: t}
	default:
		return &DefaultResolver{Target: t}
	}
}

// DefaultResolver is only capable of returning explicit hosts, named ports must match registered TCP service lookups
type DefaultResolver struct {
	Target runtime.Object
}

func (r *DefaultResolver) LookupAuthority(scheme, host string, port intstr.IntOrString) ([]string, error) {
	if host != "" {
		return resolveStatic(scheme, host, port)
	}
	return nil, fmt.Errorf("unable to resolve host")
}

// ServiceListResolver resolves authorities using the cluster IP of each Kubernetes service in the configured list
type ServiceListResolver struct {
	Services *corev1.ServiceList
}

func (r *ServiceListResolver) LookupAuthority(scheme, host string, port intstr.IntOrString) ([]string, error) {
	// If there is an explicit host, do not use the services
	if host != "" {
		return resolveStatic(scheme, host, port)
	}
	if r.Services == nil || len(r.Services.Items) == 0 {
		return nil, nil
	}

	a := make([]string, 0, len(r.Services.Items))
SERVICE_LIST:
	for i := range r.Services.Items {
		// Use the cluster IP of the service, if available
		host := r.Services.Items[i].Spec.ClusterIP
		if host == "None" || host == "" {
			// TODO Should host be the service name and namespace in this case?
			continue
		}

		// Try to match the port name to a service port name
		if port.Type == intstr.String || (port.Type == intstr.Int && port.IntVal == 0) {
			for _, sp := range r.Services.Items[i].Spec.Ports {
				if sp.Name == port.StrVal || len(r.Services.Items[i].Spec.Ports) == 1 {
					a = append(a, fmt.Sprintf("%s:%d", host, sp.Port))
					continue SERVICE_LIST
				}
			}
		}

		// As a fallback, attempt the static resolution using the per-service host name
		h, err := resolveStatic(scheme, host, port)
		if err != nil {
			return nil, err
		}
		a = append(a, h...)
	}
	return a, nil
}

// resolveStatic returns a single host with an optional numeric port
func resolveStatic(scheme, host string, port intstr.IntOrString) ([]string, error) {
	if port.Type == intstr.Int {
		if port.IntVal == 0 {
			return []string{host}, nil
		}
		return []string{host + ":" + port.String()}, nil
	}

	if port.StrVal == "" {
		if p, err := net.LookupPort("tcp", scheme); err == nil {
			return []string{host + ":" + strconv.Itoa(p)}, nil
		}
		return []string{host}, nil
	}

	if p, err := net.LookupPort("tcp", port.StrVal); err == nil {
		return []string{host + ":" + strconv.Itoa(p)}, nil
	}

	return nil, fmt.Errorf("unknown port (%s)", port.String())
}

// split splits a string on a separator (optionally removing the separator itself)
func split(s string, sep byte, cutc bool) (string, string) {
	i := strings.IndexByte(s, sep)
	if i < 0 {
		return s, ""
	}
	if cutc {
		return s[:i], s[i+1:]
	}
	return s[:i], s[i:]
}

// bracketAwareSplit is like split but it won't split between open/closing brackets
func bracketAwareSplit(s string, sep byte, cutc bool) (string, string) {
	low, high, i := strings.IndexByte(s, '{'), strings.IndexByte(s, '}'), strings.IndexByte(s, sep)
	if i > low && i < high {
		s1, s2 := split(s[high:], sep, cutc)
		return s[:high] + s1, s2
	}
	return split(s, sep, cutc)
}
