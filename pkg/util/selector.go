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

package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ client.ListOption = &Selector{}

// Selector is used to bridge the gap between the controller runtime client and the K8s API
type Selector struct{ labels.Selector }

// ApplyToList will overwrite the controller runtime client list options label selector
func (m *Selector) ApplyToList(o *client.ListOptions) { o.LabelSelector = m }

// ApplyToListOptions will overwrite the K8s list options label selector
func (m *Selector) ApplyToListOptions(o *metav1.ListOptions) { o.LabelSelector = m.String() }

// MatchingSelector exposes a K8s API label selector as a controller runtime client list option,
// normally these are not allowed to produce errors so this is a special case.
func MatchingSelector(sel *metav1.LabelSelector) (*Selector, error) {
	ls, err := metav1.LabelSelectorAsSelector(sel)
	if err != nil {
		return nil, err
	}
	return &Selector{ls}, nil
}
