package util

import (
	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Kubernetes specific configuration flags

// TODO Consider the real Kube cli-runtime?

type ConfigFlags struct {
	KubeConfig *string
}

func NewConfigFlags() *ConfigFlags {
	kubeConfig := ""
	return &ConfigFlags{KubeConfig: &kubeConfig}
}

func (f *ConfigFlags) AddFlags(flags *pflag.FlagSet) {
	if f.KubeConfig != nil {
		flags.StringVar(f.KubeConfig, "kubeconfig", *f.KubeConfig, "Path to the kubeconfig file to use for CLI requests.")
	}
}

func (f *ConfigFlags) ToRESTConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if f.KubeConfig != nil {
		loadingRules.ExplicitPath = *f.KubeConfig
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, nil).ClientConfig()
}
