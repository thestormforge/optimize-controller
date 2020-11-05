## redskyctl generate experiment

Generate an experiment

### Synopsis

Generate an experiment from an application descriptor

```
redskyctl generate experiment [flags]
```

### Options

```
  -f, --filename string          file that contains the experiment configuration
  -h, --help                     help for experiment
      --objectives stringArray   the application objectives to generate an experiment for
  -o, --output format            output format. one of: json|yaml (default "yaml")
  -r, --resources stringArray    additional resources to consider
  -s, --scenario string          the application scenario to generate an experiment for
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

