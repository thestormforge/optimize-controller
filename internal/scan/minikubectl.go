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

package scan

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// minikubectl is just a miniature in-process kubectl for us to use to avoid
// a binary dependency on the tool itself.
type minikubectl struct {
	ConfigFlags          *genericclioptions.ConfigFlags
	ResourceBuilderFlags *genericclioptions.ResourceBuilderFlags
	PrintFlags           *genericclioptions.PrintFlags
	IgnoreNotFound       bool

	restConfig *rest.Config
	lock       sync.Mutex
}

// newMinikubectl creates a new minikubectl, the empty state is not usable.
func newMinikubectl() *minikubectl {
	outputFormat := ""
	return &minikubectl{
		ConfigFlags: genericclioptions.NewConfigFlags(true),
		ResourceBuilderFlags: genericclioptions.NewResourceBuilderFlags().
			WithLabelSelector("").
			WithAll(true).
			WithLatest(),
		PrintFlags: &genericclioptions.PrintFlags{
			JSONYamlPrintFlags: genericclioptions.NewJSONYamlPrintFlags(),
			OutputFormat:       &outputFormat,
		},
	}
}

// AddFlags configures the supplied flag set with the recognized flags.
func (k *minikubectl) AddFlags(flags *pflag.FlagSet) {
	k.ConfigFlags.AddFlags(flags)
	k.ResourceBuilderFlags.AddFlags(flags)

	// PrintFlags somehow is tied to Cobra...
	flags.StringVarP(k.PrintFlags.OutputFormat, "output", "o", *k.PrintFlags.OutputFormat, "")

	// Don't bother with usage strings here, we aren't showing help to anyone
	flags.BoolVar(&k.IgnoreNotFound, "ignore-not-found", k.IgnoreNotFound, "")
}

// Complete validates we can execute against the supplied arguments.
func (k *minikubectl) Complete(args []string) error {
	if len(args) == 0 || args[0] != "get" {
		return fmt.Errorf("minikubectl only supports get")
	}

	return nil
}

// Run executes the supplied arguments and returns the output as bytes.
func (k *minikubectl) Run(args []string) ([]byte, error) {
	v := k.ResourceBuilderFlags.ToBuilder(k, args[1:]).Do()

	// Create a printer to dump the objects
	printer, err := k.PrintFlags.ToPrinter()
	if err != nil {
		return nil, err
	}

	// Use the printer to render everything into a byte buffer
	var b bytes.Buffer
	err = v.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			if k.IgnoreNotFound && apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		return printer.PrintObj(info.Object, &b)
	})
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

// ToRawKubeConfigLoader just defers to the configuration flags.
func (k *minikubectl) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return &namespaceOverrideClientConfig{
		Delegate:          k.ConfigFlags.ToRawKubeConfigLoader(),
		NamespaceOverride: k.ConfigFlags.Namespace,
	}
}

// ToRESTConfig lazily loads a REST configuration.
func (k *minikubectl) ToRESTConfig() (*rest.Config, error) {
	k.lock.Lock()
	defer k.lock.Unlock()

	var err error
	if k.restConfig != nil {
		k.restConfig, err = k.ToRawKubeConfigLoader().ClientConfig()

	}
	return k.restConfig, err
}

// ToDiscoveryClient returns an in-memory cached discovery instance instead of an on-disk cached instance.
func (k *minikubectl) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := k.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	return memory.NewMemCacheClient(discoveryClient), nil
}

// ToRESTMapper does the exact same thing as the ConfigFlag implementation, just with a different discovery client.
func (k *minikubectl) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := k.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

type namespaceOverrideClientConfig struct {
	Delegate          clientcmd.ClientConfig
	NamespaceOverride *string
}

func (c *namespaceOverrideClientConfig) Namespace() (string, bool, error) {
	if c.NamespaceOverride != nil && *c.NamespaceOverride != "" {
		return *c.NamespaceOverride, true, nil
	}
	return c.Delegate.Namespace()
}

// NOTE: Because the interface ClientConfig has a function ClientConfig we cannot simply embed ClientConfig

func (c *namespaceOverrideClientConfig) RawConfig() (clientcmdapi.Config, error) {
	return c.Delegate.RawConfig()
}

func (c *namespaceOverrideClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return c.Delegate.ConfigAccess()
}

func (c *namespaceOverrideClientConfig) ClientConfig() (*restclient.Config, error) {
	return c.Delegate.ClientConfig()
}
