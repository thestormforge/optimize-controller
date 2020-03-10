## redskyctl get

Display a Red Sky resource

### Synopsis

Get Red Sky resources from the remote server

```
redskyctl get TYPE NAME... [flags]
```

### Options

```
      --chunk-size int       Fetch large lists in chunks rather then all at once. (default 500)
  -h, --help                 help for get
      --no-headers           Don't print headers.
  -o, --output format        Output format. One of: json|yaml|name|wide|csv
  -l, --selector query       Selector (label query) to filter on.
      --show-labels          When printing, show all labels as the last column.
      --sort-by expression   Sort list types using this JSONPath expression.
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

