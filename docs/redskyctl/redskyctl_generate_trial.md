## redskyctl generate trial

Generate experiment trials

### Synopsis

Generate a trial from an experiment manifest

```
redskyctl generate trial [flags]
```

### Options

```
  -A, --assign stringToString   Assign an explicit value to a parameter. (default [])
      --default string          Select the behavior for default values; one of: none|min|max|rand.
  -f, --filename string         File that contains the experiment to generate trials for.
  -h, --help                    help for trial
      --interactive             Allow interactive prompts for unspecified parameter assignments.
  -o, --output format           Output format. One of: json|yaml (default "yaml")
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

