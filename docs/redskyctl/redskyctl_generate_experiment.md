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
  -o, --output format            Output format. One of: json|yaml (default "yaml")
  -r, --resources stringArray    additional resources to consider
  -s, --scenario string          the application scenario to generate an experiment for
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

