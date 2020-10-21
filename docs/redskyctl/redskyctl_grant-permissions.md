## redskyctl grant-permissions

Grant permissions

### Synopsis

Grant the Red Sky Controller permissions

```
redskyctl grant-permissions [flags]
```

### Options

```
      --create-trial-namespace   include trial namespace creation permissions
  -h, --help                     help for grant-permissions
      --include-manager          bind manager to matching namespaces
      --ns-selector string       bind to matching namespaces
      --skip-default             skip default permissions
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

