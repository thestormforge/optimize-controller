## redskyctl check server

Check the server

### Synopsis

Check the Red Sky Ops server

```
redskyctl check server [flags]
```

### Options

```
      --dry-run          Generate experiment JSON to stdout.
      --fail             Report an experiment failure instead of generated values.
  -h, --help             help for server
      --invalid          Skip client side validity checks (server enforcement).
      --metrics int      Specify the number of experiment metrics to generate (1 or 2).
      --parameters int   Specify the number of experiment parameters to generate (1 - 20).
```

### Options inherited from parent commands

```
      --context string        The name of the redskyconfig context to use. NOT THE KUBE CONTEXT.
      --kubeconfig string     Path to the kubeconfig file to use for CLI requests.
  -n, --namespace string      If present, the namespace scope for this CLI request.
      --redskyconfig string   Path to the redskyconfig file to use.
```

### SEE ALSO

* [redskyctl check](redskyctl_check.md)	 - Run a consistency check

