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

import "fmt"

// Reader exposes information from a configuration
type Reader interface {
	// ServerName returns the server name for the specified context
	ServerName(contextName string) (string, error)
	// Server returns the named server configuration
	Server(name string) (Server, error)
	// AuthorizationName returns authorization name for the specified context
	AuthorizationName(contextName string) (string, error)
	// Authorization returns the named authorization configuration
	Authorization(name string) (Authorization, error)
	// ClusterName returns cluster name for the specified context
	ClusterName(contextName string) (string, error)
	// Cluster returns the named cluster configuration
	Cluster(name string) (Cluster, error)
	// ControllerName returns controller name for the specified context (derived via the cluster)
	ControllerName(contextName string) (string, error)
	// Controller returns the named controller configuration
	Controller(name string) (Controller, error)
	// ContextName returns current context name
	ContextName() string
	// Context returns the named context configuration
	Context(name string) (Context, error)
}

// TODO All these things that take a reader should be aggregated onto a single type

// CurrentServer returns the current server configuration
func CurrentServer(r Reader) (Server, error) {
	n, err := r.ServerName(r.ContextName())
	if err != nil {
		return Server{}, err
	}
	return r.Server(n)
}

// CurrentAuthorization returns the current authorization configuration
func CurrentAuthorization(r Reader) (Authorization, error) {
	n, err := r.AuthorizationName(r.ContextName())
	if err != nil {
		return Authorization{}, err
	}
	return r.Authorization(n)
}

// CurrentCluster returns the current cluster configuration
func CurrentCluster(r Reader) (Cluster, error) {
	n, err := r.ClusterName(r.ContextName())
	if err != nil {
		return Cluster{}, err
	}
	return r.Cluster(n)
}

// CurrentController returns the current controller configuration
func CurrentController(r Reader) (Controller, error) {
	n, err := r.ControllerName(r.ContextName())
	if err != nil {
		return Controller{}, err
	}
	return r.Controller(n)
}

// Minify creates a new configuration using only the data available through the reader
func Minify(r Reader) (*Config, error) {
	cfg := &Config{CurrentContext: r.ContextName()}
	ctx, err := r.Context(cfg.CurrentContext)
	if err != nil {
		return nil, err
	}
	cfg.Contexts = append(cfg.Contexts, NamedContext{Name: cfg.CurrentContext, Context: ctx})

	controllerName, err := r.ControllerName(cfg.CurrentContext)
	if err != nil {
		return nil, err
	}
	ctrl, err := r.Controller(controllerName)
	if err != nil {
		return nil, err
	}
	cfg.Controllers = append(cfg.Controllers, NamedController{Name: controllerName, Controller: ctrl})

	clusterName, err := r.ClusterName(cfg.CurrentContext)
	if err != nil {
		return nil, err
	}
	cstr, err := r.Cluster(clusterName)
	if err != nil {
		return nil, err
	}
	cfg.Clusters = append(cfg.Clusters, NamedCluster{Name: clusterName, Cluster: cstr})

	authorizationName, err := r.AuthorizationName(cfg.CurrentContext)
	if err != nil {
		return nil, err
	}
	az, err := r.Authorization(authorizationName)
	if err != nil {
		return nil, err
	}
	cfg.Authorizations = append(cfg.Authorizations, NamedAuthorization{Name: authorizationName, Authorization: az})

	serverName, err := r.ServerName(cfg.CurrentContext)
	if err != nil {
		return nil, err
	}
	srv, err := r.Server(serverName)
	if err != nil {
		return nil, err
	}
	cfg.Servers = append(cfg.Servers, NamedServer{Name: serverName, Server: srv})

	return cfg, nil
}

// TODO Instead of defaultReader, just have RedSkyConfig implement Reader?

var _ Reader = &defaultReader{}

type defaultReader struct {
	cfg *Config
}

func newNotFoundError(kind, name string) error {
	return fmt.Errorf("config: %s '%s' not found", kind, name)
}

func (d *defaultReader) name(contextName string, f func(*Context) string) (string, error) {
	ctx := findContext(d.cfg.Contexts, contextName)
	if ctx == nil {
		return "", newNotFoundError("context", contextName)
	}

	name := f(ctx)
	if name == "" {
		return "", fmt.Errorf("config: name not set")
	}
	return name, nil
}

func (d *defaultReader) ServerName(contextName string) (string, error) {
	return d.name(contextName, func(ctx *Context) string { return ctx.Server })
}

func (d *defaultReader) Server(name string) (Server, error) {
	srv := findServer(d.cfg.Servers, name)
	if srv == nil {
		return Server{}, newNotFoundError("server", name)
	}
	return *srv, nil
}

func (d *defaultReader) AuthorizationName(contextName string) (string, error) {
	return d.name(contextName, func(ctx *Context) string { return ctx.Authorization })
}

func (d *defaultReader) Authorization(name string) (Authorization, error) {
	az := findAuthorization(d.cfg.Authorizations, name)
	if az == nil {
		return Authorization{}, newNotFoundError("authorization", name)
	}
	return *az, nil
}

func (d *defaultReader) ClusterName(contextName string) (string, error) {
	return d.name(contextName, func(ctx *Context) string { return ctx.Cluster })
}

func (d *defaultReader) Cluster(name string) (Cluster, error) {
	cstr := findCluster(d.cfg.Clusters, name)
	if cstr == nil {
		return Cluster{}, newNotFoundError("cluster", name)
	}
	return *cstr, nil
}

func (d *defaultReader) ControllerName(contextName string) (string, error) {
	return d.name(contextName, func(ctx *Context) string {
		cstr := findCluster(d.cfg.Clusters, ctx.Cluster)
		if cstr == nil {
			return ""
		}
		return cstr.Controller
	})
}

func (d *defaultReader) Controller(name string) (Controller, error) {
	ctrl := findController(d.cfg.Controllers, name)
	if ctrl == nil {
		return Controller{}, newNotFoundError("controller", name)
	}
	return *ctrl, nil
}

func (d *defaultReader) ContextName() string {
	return d.cfg.CurrentContext
}

func (d *defaultReader) Context(name string) (Context, error) {
	ctx := findContext(d.cfg.Contexts, name)
	if ctx == nil {
		return Context{}, newNotFoundError("context", name)
	}
	return *ctx, nil
}
