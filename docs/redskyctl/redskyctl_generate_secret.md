## redskyctl generate secret

Generate Red Sky Ops authorization

### Synopsis

Generate authorization secret for Red Sky Ops

```
redskyctl generate secret [flags]
```

### Options

```
      --client-name string   Client name to use for registration.
  -h, --help                 help for secret
  -o, --output format        Output format. One of: json|yaml|helm (default "yaml")
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

