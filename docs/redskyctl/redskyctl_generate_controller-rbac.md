## redskyctl generate controller-rbac

Generate Red Sky Ops permissions

### Synopsis

Generate RBAC for Red Sky Ops

```
redskyctl generate controller-rbac [flags]
```

### Options

```
      --create-trial-namespace   include trial namespace creation permissions
  -h, --help                     help for controller-rbac
      --include-manager          bind manager to matching namespaces
      --ns-selector string       bind to matching namespaces
  -o, --output format            output format. one of: json|yaml (default "yaml")
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

* [redskyctl generate](redskyctl_generate.md)	 - Generate Red Sky Ops objects

