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

	"github.com/spf13/pflag"
	"k8s.io/client-go/discovery/cached/memory"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Minikubectl is just a miniature in-process kubectl for us to use to avoid
// a binary dependency on the tool itself.
type Minikubectl struct {
	*genericclioptions.ConfigFlags
	*genericclioptions.ResourceBuilderFlags
	*genericclioptions.PrintFlags
	IgnoreNotFound bool
	Manager        *MiniManager
}

// MiniManager is used to use what we can from controller-runtime.Manager and satisfy the
// RESTClientGetter interface so we dont need to use another kube client
type MiniManager struct {
	Config *rest.Config
	Mapper meta.RESTMapper
}

// ToRESTConfig implements the RESTClientGetter interface
func (m *MiniManager) ToRESTConfig() (*rest.Config, error) {
	return m.Config, nil
}

// ToDiscoveryClient implements the RESTClientGetter interface
func (m *MiniManager) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return memory.NewMemCacheClient(discovery.NewDiscoveryClientForConfigOrDie(m.Config)), nil
}

// ToRESTMapper implements the RESTClientGetter interface
func (m *MiniManager) ToRESTMapper() (meta.RESTMapper, error) {
	return m.Mapper, nil
}

// ToRawKubeConfigLoader implements the RESTClientGetter interface
func (m *MiniManager) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	// Only ClientConfig.Namespace() is used from cli builder, so we need something
	// dumb here
	return &dumbConfig{}
}

// dumbConfig implments the ClientConfig interface
type dumbConfig struct{}

func (d *dumbConfig) RawConfig() (clientcmdapi.Config, error) { return clientcmdapi.Config{}, nil }
func (d *dumbConfig) ClientConfig() (*rest.Config, error)     { return nil, nil }
func (d *dumbConfig) Namespace() (string, bool, error)        { return "default", false, nil }
func (d *dumbConfig) ConfigAccess() clientcmd.ConfigAccess    { return nil }

// NewMinikubectl creates a new Minikubectl, the empty state is not usable.
func NewMinikubectl() *Minikubectl {
	outputFormat := ""
	return &Minikubectl{
		ConfigFlags: genericclioptions.NewConfigFlags(false),
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
func (m *Minikubectl) AddFlags(flags *pflag.FlagSet) {
	m.ConfigFlags.AddFlags(flags)
	m.ResourceBuilderFlags.AddFlags(flags)

	// PrintFlags somehow is tied to Cobra...
	flags.StringVarP(m.PrintFlags.OutputFormat, "output", "o", *m.PrintFlags.OutputFormat, "")

	// Don't bother with usage strings here, we aren't showing help to anyone
	flags.BoolVar(&m.IgnoreNotFound, "ignore-not-found", m.IgnoreNotFound, "")
}

// Complete validates we can execute against the supplied arguments.
func (m *Minikubectl) Complete(args []string) error {
	if len(args) == 0 || args[0] != "get" {
		return fmt.Errorf("Minikubectl only supports get")
	}

	return nil
}

// Run executes the supplied arguments and returns the output as bytes.
func (m *Minikubectl) Run(args []string) ([]byte, error) {
	var rcg genericclioptions.RESTClientGetter
	if m.Manager != nil {
		rcg = m.Manager
	} else {
		rcg = m.ConfigFlags
	}

	r := m.ResourceBuilderFlags.ToBuilder(rcg, args[1:])
	v := r.Do()

	// Create a printer to dump the objects
	printer, err := m.PrintFlags.ToPrinter()
	if err != nil {
		return nil, err
	}

	// Use the printer to render everything into a byte buffer
	var b bytes.Buffer
	err = v.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			if m.IgnoreNotFound && apierrors.IsNotFound(err) {
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
