## redskyctl generate bootstrap-cluster-role

Generate Red Sky Ops permissions

### Synopsis

Generate RBAC for Red Sky Ops

```
redskyctl generate bootstrap-cluster-role [flags]
```

### Options

```
      --create-trial-namespace   Include trial namespace creation permissions.
  -h, --help                     help for bootstrap-cluster-role
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

