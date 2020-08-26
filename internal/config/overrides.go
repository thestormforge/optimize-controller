/*
Copyright 2020 GramLabs, Inc.

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

package config

// Overrides represent information which can be overridden in the configuration
type Overrides struct {
	// Environment overrides the execution environment name
	Environment string
	// Context overrides the current Red Sky context name (_not_ the KubeConfig context)
	Context string
	// SystemNamespace overrides the current controller namespace (_not_ the Kube namespace)
	SystemNamespace string
	// ServerIdentifier overrides the current server's identifier and Red Sky endpoints. Using this override, it is not possible to specify individual endpoint locations.
	ServerIdentifier string
	// ServerIssuer overrides the current server's authorization server issuer. Using this override, it is not possible to specify individual endpoint locations.
	ServerIssuer string
	// Credential overrides the current authorization
	Credential ClientCredential
	// KubeConfig overrides the current cluster's kubeconfig file
	KubeConfig string
	// Namespace overrides the current cluster's default namespace
	Namespace string
}

var _ Reader = &overrideReader{}

type overrideReader struct {
	overrides *Overrides
	delegate  Reader
}

func (o *overrideReader) ServerName(contextName string) (string, error) {
	return o.delegate.ServerName(contextName)
}

func (o *overrideReader) Server(name string) (Server, error) {
	srv, err := o.delegate.Server(name)
	if err != nil {
		return srv, err
	}

	if o.overrides.ServerIdentifier != "" {
		mergeString(&srv.Identifier, o.overrides.ServerIdentifier)
		srv.RedSky = RedSkyServer{}
		srv.Authorization.RegistrationEndpoint = ""
	}

	if o.overrides.ServerIssuer != "" {
		srv.Authorization = AuthorizationServer{Issuer: o.overrides.ServerIssuer}
	}

	if o.overrides.ServerIdentifier != "" || o.overrides.ServerIssuer != "" {
		if err := defaultServerEndpoints(&srv); err != nil {
			return Server{}, err
		}
	}

	return srv, nil
}

func (o *overrideReader) AuthorizationName(contextName string) (string, error) {
	return o.delegate.AuthorizationName(contextName)
}

func (o *overrideReader) Authorization(name string) (Authorization, error) {
	if o.overrides.Credential.ClientID != "" && o.overrides.Credential.ClientSecret != "" {
		cc := o.overrides.Credential
		return Authorization{Credential: Credential{ClientCredential: &cc}}, nil
	}

	return o.delegate.Authorization(name)
}

func (o *overrideReader) ClusterName(contextName string) (string, error) {
	return o.delegate.ClusterName(contextName)
}

func (o *overrideReader) Cluster(name string) (Cluster, error) {
	cstr, err := o.delegate.Cluster(name)
	if err == nil {
		mergeString(&cstr.KubeConfig, o.overrides.KubeConfig)
		mergeString(&cstr.Namespace, o.overrides.Namespace)
	}
	return cstr, err
}

func (o *overrideReader) ControllerName(contextName string) (string, error) {
	return o.delegate.ControllerName(contextName)
}

func (o *overrideReader) Controller(name string) (Controller, error) {
	ctrl, err := o.delegate.Controller(name)
	if err == nil {
		mergeString(&ctrl.Namespace, o.overrides.SystemNamespace)
	}
	return ctrl, err
}

func (o *overrideReader) ContextName() string {
	if o.overrides.Context != "" {
		return o.overrides.Context
	}
	return o.delegate.ContextName()
}

func (o *overrideReader) Context(name string) (Context, error) {
	return o.delegate.Context(name)
}
