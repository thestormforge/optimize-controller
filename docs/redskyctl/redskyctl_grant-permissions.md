## redskyctl grant-permissions

Grant permissions

### Synopsis

Grant the Red Sky Controller permissions

```
redskyctl grant-permissions [flags]
```

### Options

```
      --create-trial-namespace   Include trial namespace creation permissions.
  -h, --help                     help for grant-permissions
      --include-manager          Bind manager to matching namespaces.
      --ns-selector string       Bind to matching namespaces.
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

* [redskyctl](redskyctl.md)	 - Kubernetes Exploration

