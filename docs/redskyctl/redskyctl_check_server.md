## redskyctl check server

Check the server

### Synopsis

Check the Red Sky Ops server

```
redskyctl check server [flags]
```

### Options

```
      --dry-run          generate experiment JSON to stdout
      --fail             report an experiment failure instead of generated values
  -h, --help             help for server
      --invalid          skip client side validity checks (server enforcement)
      --metrics int      specify the number of experiment metrics to generate (1 or 2)
      --parameters int   specify the number of experiment parameters to generate (1 - 20)
```

### Options inherited from parent commands

```
      --context name        the name of the redskyconfig context to use, NOT THE KUBE CONTEXT
      --kubeconfig file     path to the kubeconfig file to use for CLI requests
  -n, --namespace string    if present, the namespace scope for this CLI request
      --redskyconfig file   path to the redskyconfig file to use
```

### SEE ALSO

* [redskyctl check](redskyctl_check.md)	 - Run a consistency check

