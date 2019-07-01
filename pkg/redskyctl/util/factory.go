package util

import (
	redsky "github.com/gramLabs/redsky/pkg/api/redsky/v1alpha1"
	"k8s.io/client-go/kubernetes"
)

type Factory interface {
	KubernetesClientSet() (*kubernetes.Clientset, error)
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

func (f *factoryImpl) RedSkyAPI() (redsky.API, error) {
	c, err := f.serverFlags.ToClientConfig()
	if err != nil {
		return nil, err
	}
	return redsky.NewForConfig(c)
}
