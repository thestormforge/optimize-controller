## redskyctl kustomize config

Configure Kustomize transformers

### Synopsis

Configure Kustomize transformers for Red Sky types

```
redskyctl kustomize config [flags]
```

### Options

```
  -f, --filename file    file to write the configuration to (relative to the Kustomize root, if specified)
  -h, --help             help for config
  -k, --kustomize root   Kustomize root to update
```

### Options inherited from parent commands

```
      --context string        The name of the redskyconfig context to use. NOT THE KUBE CONTEXT.
      --kubeconfig string     Path to the kubeconfig file to use for CLI requests.
  -n, --namespace string      If present, the namespace scope for this CLI request.
      --redskyconfig string   Path to the redskyconfig file to use.
```

### SEE ALSO

* [redskyctl kustomize](redskyctl_kustomize.md)	 - Kustomize integrations

