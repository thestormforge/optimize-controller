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
	"github.com/redskyops/redskyops-controller/internal/config"
	redskyclient "github.com/redskyops/redskyops-controller/redskyapi"
	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	// Initialize all known client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// Kubernetes specific configuration flags

const (
	flagKubeconfig = "kubeconfig"
	flagNamespace  = "namespace"
)

type ConfigFlags struct {
	cfg *config.RedSkyConfig
}

func NewConfigFlags(cfg *config.RedSkyConfig) *ConfigFlags {
	return &ConfigFlags{
		cfg: cfg,
	}
}

func (f *ConfigFlags) AddFlags(flags *pflag.FlagSet) {
	// NOTE: There is no override for the kubeconfig cluster because it conflicts with the Red Sky config concept of a cluster

	flags.StringVar(&f.cfg.Overrides.KubeConfig, flagKubeconfig, "", "Path to the kubeconfig file to use for CLI requests.")
	flags.StringVarP(&f.cfg.Overrides.Namespace, flagNamespace, "n", "", "If present, the namespace scope for this CLI request.")
}

func (f *ConfigFlags) ToRESTConfig() (*rest.Config, error) {
	return f.ToRawKubeConfigLoader().ClientConfig()
}

func (f *ConfigFlags) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}

	if cstr, err := config.CurrentCluster(f.cfg.Reader()); err == nil {
		loadingRules.ExplicitPath = cstr.KubeConfig
		overrides.CurrentContext = cstr.Context
		overrides.Context.Namespace = cstr.Namespace
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}

func (f *ConfigFlags) ToClientConfig() (redskyclient.Config, error) {
	return f.cfg, nil
}
