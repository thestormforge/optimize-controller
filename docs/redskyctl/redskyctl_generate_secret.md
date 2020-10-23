## redskyctl generate secret

Generate Red Sky Ops authorization

### Synopsis

Generate authorization secret for Red Sky Ops

```
redskyctl generate secret [flags]
```

### Options

```
      --client-name string   client name to use for registration
  -h, --help                 help for secret
  -o, --output format        output format. one of: json|yaml|helm (default "yaml")
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

