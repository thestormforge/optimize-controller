## redskyctl generate controller-rbac

Generate Red Sky Ops permissions

### Synopsis

Generate RBAC for Red Sky Ops

```
redskyctl generate controller-rbac [flags]
```

### Options

```
      --create-trial-namespace   Include trial namespace creation permissions.
  -h, --help                     help for controller-rbac
      --include-manager          Bind manager to matching namespaces.
      --ns-selector string       Bind to matching namespaces.
  -o, --output format            Output format. One of: json|yaml (default "yaml")
      --skip-default             Skip default permissions.
```

### Options inherited from parent commands

```
      --context string        The name of the redskyconfig context to use. NOT THE KUBE CONTEXT.
      --kubeconfig string     Path to the kubeconfig file to use for CLI requests.
  -n, --namespace string      If present, the namespace scope for this CLI request.
      --redskyconfig string   Path to the redskyconfig file to use.
```

### SEE ALSO

* [redskyctl generate](redskyctl_generate.md)	 - Generate Red Sky Ops objects

