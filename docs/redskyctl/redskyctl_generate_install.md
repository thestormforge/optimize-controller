## redskyctl generate install

Generate Red Sky Ops manifests

### Synopsis

Generate installation manifests for Red Sky Ops

```
redskyctl generate install [flags]
```

### Options

```
      --bootstrap-role       Create the bootstrap role.
      --extra-permissions    Generate permissions required for features like namespace creation.
  -h, --help                 help for install
      --ns-selector string   Create namespaced role bindings to matching namespaces.
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

