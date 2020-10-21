## redskyctl init

Install to a cluster

### Synopsis

Install Red Sky Ops to a cluster

```
redskyctl init [flags]
```

### Options

```
      --bootstrap-role       create the bootstrap role (default true)
      --extra-permissions    generate permissions required for features like namespace creation
  -h, --help                 help for init
      --ns-selector string   create namespaced role bindings to matching namespaces
      --wait                 wait for resources to be established before returning
```

### Options inherited from parent commands

```
      --context name        the name of the redskyconfig context to use, NOT THE KUBE CONTEXT
      --kubeconfig file     path to the kubeconfig file to use for CLI requests
  -n, --namespace string    if present, the namespace scope for this CLI request
      --redskyconfig file   path to the redskyconfig file to use
```

### SEE ALSO

* [redskyctl](redskyctl.md)	 - Kubernetes Exploration

