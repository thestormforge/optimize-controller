## redskyctl generate trial

Generate experiment trials

### Synopsis

Generate a trial from an experiment manifest

```
redskyctl generate trial [flags]
```

### Options

```
  -A, --assign stringToString   assign an explicit value to a parameter (default [])
      --default string          select the behavior for default values; one of: none|min|max|rand
  -f, --filename string         file that contains the experiment to generate trials for
  -h, --help                    help for trial
      --interactive             allow interactive prompts for unspecified parameter assignments
  -l, --labels string           comma separated labels to apply to the trial
  -o, --output format           output format. one of: json|yaml (default "yaml")
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

