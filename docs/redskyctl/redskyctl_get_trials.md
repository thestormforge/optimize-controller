## redskyctl get trials

Display a list of trial for an experiment

### Synopsis

Prints a list of trials for an experiment using a tabular format by default

```
redskyctl get trials NAME [flags]
```

### Options

```
      --chunk-size int    Fetch large lists in chunks rather then all at once. (default 500)
  -h, --help              help for trials
      --no-headers        Don't print headers.
  -o, --output string     Output format. One of: json|yaml|name|csv
  -l, --selector string   Selector to filter on.
      --show-labels       When printing, show all labels as the last column.
      --sort-by string    Sort list types using this JSONPath expression.
```

### Options inherited from parent commands

```
      --context string        The name of the redskyconfig context to use. NOT THE KUBE CONTEXT.
      --kubeconfig string     Path to the kubeconfig file to use for CLI requests.
  -n, --namespace string      If present, the namespace scope for this CLI request.
      --redskyconfig string   Path to the redskyconfig file to use.
```

### SEE ALSO

* [redskyctl get](redskyctl_get.md)	 - Display a Red Sky resource

