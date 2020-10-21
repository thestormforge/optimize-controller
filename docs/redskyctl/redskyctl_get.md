## redskyctl get

Display a Red Sky resource

### Synopsis

Get Red Sky resources from the remote server

```
redskyctl get (TYPE NAME | TYPE/NAME ...) [flags]
```

### Options

```
  -A, --all                  include all resources
      --chunk-size int       fetch large lists in chunks rather then all at once (default 500)
  -h, --help                 help for get
      --no-headers           don't print headers
  -o, --output format        output format. one of: json|yaml|name|wide|csv
  -l, --selector query       selector (label query) to filter on
      --show-labels          when printing, show all labels as the last column
      --sort-by expression   sort list types using this JSONPath expression
```

### Options inherited from parent commands

```
      --context name        the name of the redskyconfig context to use, NOT THE KUBE CONTEXT
      --kubeconfig file     path to the kubeconfig file to use for CLI requests
  -n, --namespace string    if present, the namespace scope for this CLI request
      --redskyconfig file   path to the redskyconfig file to use
```

### SEE ALSO

* [redskyctl](redskyctl.md)	 - Kubernetes Exploration

