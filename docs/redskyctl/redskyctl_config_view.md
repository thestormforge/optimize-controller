## redskyctl config view

View the configuration file

### Synopsis

View the Red Sky Ops configuration file

```
redskyctl config view [flags]
```

### Options

```
  -h, --help            help for view
      --minify          reduce information to effective values
  -o, --output format   output format. One of: yaml|json (default "yaml")
      --raw             display the raw configuration file without merging
```

### Options inherited from parent commands

```
      --context name        the name of the redskyconfig context to use, NOT THE KUBE CONTEXT
      --kubeconfig file     path to the kubeconfig file to use for CLI requests
  -n, --namespace string    if present, the namespace scope for this CLI request
      --redskyconfig file   path to the redskyconfig file to use
```

### SEE ALSO

* [redskyctl config](redskyctl_config.md)	 - Work with the configuration file

