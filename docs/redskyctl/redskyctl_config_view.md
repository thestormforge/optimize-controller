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
      --minify          Reduce information to effective values.
  -o, --output format   Output format. One of: yaml|json (default "yaml")
      --raw             Display the raw configuration file without merging.
```

### Options inherited from parent commands

```
      --context string        The name of the redskyconfig context to use. NOT THE KUBE CONTEXT.
      --kubeconfig string     Path to the kubeconfig file to use for CLI requests.
  -n, --namespace string      If present, the namespace scope for this CLI request.
      --redskyconfig string   Path to the redskyconfig file to use.
```

### SEE ALSO

* [redskyctl config](redskyctl_config.md)	 - Work with the configuration file

