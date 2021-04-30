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

package internal

import "fmt"

type Version string

func NewVersion(v interface{}) *Version {
	result := Version(fmt.Sprintf("%s", v))
	return &result
}

func (v *Version) Available() bool {
	return v != nil && *v != ""
}

func (v *Version) String() string {
	if v == nil {
		return ""
	}
	return string(*v)
}

// AuthorizationStatus represents different states of authorization for an API. Note that
// just because an authorization is available, it does not guarantee what (if any) actions
// can be performed.
type AuthorizationStatus int

const (
	// AuthorizationUnknown means the authorization status could not be determined.
	AuthorizationUnknown AuthorizationStatus = iota
	// AuthorizationInvalid means the end user is not currently authorized.
	AuthorizationInvalid
	// AuthorizationValid means the end user is authorized to perform at least some operations.
	AuthorizationValid
	// AuthorizationInvalidIgnored means the end user is not authorized to do anything but would like to try anyway.
	AuthorizationInvalidIgnored
)

func (s AuthorizationStatus) Allowed() bool {
	return s == AuthorizationValid || s == AuthorizationInvalidIgnored
}
