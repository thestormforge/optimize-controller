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
	redsky "github.com/redskyops/k8s-experiment/pkg/api/redsky/v1alpha1"
	redskykube "github.com/redskyops/k8s-experiment/pkg/kubernetes"
	"k8s.io/client-go/kubernetes"
)

type Factory interface {
	KubernetesClientSet() (*kubernetes.Clientset, error)
	RedSkyClientSet() (*redskykube.Clientset, error)
	RedSkyAPI() (redsky.API, error)
}

var _ Factory = &factoryImpl{}

func NewFactory(cf *ConfigFlags, sf *ServerFlags) Factory {
	if cf == nil {
		panic("attempt to create factory with nil config flags")
	}
	if sf == nil {
		panic("attempt to create factory with nil server flags")
	}
	return &factoryImpl{configFlags: cf, serverFlags: sf}
}

type factoryImpl struct {
	configFlags *ConfigFlags
	serverFlags *ServerFlags
}

func (f *factoryImpl) KubernetesClientSet() (*kubernetes.Clientset, error) {
	c, err := f.configFlags.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(c)
}

func (f *factoryImpl) RedSkyClientSet() (*redskykube.Clientset, error) {
	c, err := f.configFlags.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	return redskykube.NewForConfig(c)
}

func (f *factoryImpl) RedSkyAPI() (redsky.API, error) {
	c, err := f.serverFlags.ToClientConfig()
	if err != nil {
		return nil, err
	}
	return redsky.NewForConfig(c)
}
