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
	"github.com/spf13/pflag"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Kubernetes specific configuration flags

// TODO Consider the real Kube cli-runtime?

const (
	flagKubeconfig = "kubeconfig"
	flagContext    = "context"
	flagNamespace  = "namespace"
)

type ConfigFlags struct {
	KubeConfig *string
	Context    *string
	Namespace  *string
}

func NewConfigFlags() *ConfigFlags {
	return &ConfigFlags{
		KubeConfig: stringptr(""),
		Context:    stringptr(""),
		Namespace:  stringptr(""),
	}
}

func (f *ConfigFlags) AddFlags(flags *pflag.FlagSet) {
	if f.KubeConfig != nil {
		flags.StringVar(f.KubeConfig, flagKubeconfig, *f.KubeConfig, "Path to the kubeconfig file to use for CLI requests.")
	}
	if f.Context != nil {
		flags.StringVar(f.Context, flagContext, *f.Context, "The name of the kubeconfig context to use.")
	}
	if f.Namespace != nil {
		flags.StringVarP(f.Namespace, flagNamespace, "n", *f.Namespace, "If present, the namespace scope for this CLI request.")
	}
}

func (f *ConfigFlags) ToRESTConfig() (*rest.Config, error) {
	return f.ToRawKubeConfigLoader().ClientConfig()
}

func (f *ConfigFlags) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if f.KubeConfig != nil {
		loadingRules.ExplicitPath = *f.KubeConfig
	}
	overrides := &clientcmd.ConfigOverrides{}
	if f.Context != nil {
		overrides.CurrentContext = *f.Context
	}
	if f.Namespace != nil {
		overrides.Context.Namespace = *f.Namespace
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}
