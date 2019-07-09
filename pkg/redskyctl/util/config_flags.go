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
)

type ConfigFlags struct {
	KubeConfig *string
}

func NewConfigFlags() *ConfigFlags {
	return &ConfigFlags{
		KubeConfig: stringptr(""),
	}
}

func (f *ConfigFlags) AddFlags(flags *pflag.FlagSet) {
	if f.KubeConfig != nil {
		flags.StringVar(f.KubeConfig, flagKubeconfig, *f.KubeConfig, "Path to the kubeconfig file to use for CLI requests.")
	}
}

func (f *ConfigFlags) ToRESTConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if f.KubeConfig != nil {
		loadingRules.ExplicitPath = *f.KubeConfig
	}
	overrides := &clientcmd.ConfigOverrides{}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
}
