## redskyctl generate install

Generate Red Sky Ops manifests

### Synopsis

Generate installation manifests for Red Sky Ops

```
redskyctl generate install [flags]
```

### Options

```
      --bootstrap-role         create the bootstrap role
      --extra-permissions      generate permissions required for features like namespace creation
  -h, --help                   help for install
      --ns-selector string     create namespaced role bindings to matching namespaces
      --output-dir directory   write files to a directory instead of stdout
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

