## redskyctl suggest

Suggest assignments

### Synopsis

Suggest assignments for a new trial run

```
redskyctl suggest NAME [flags]
```

### Options

```
  -A, --assign stringToString   assign an explicit value to a parameter (default [])
      --default string          select the behavior for default values; one of: none|min|max|rand
  -h, --help                    help for suggest
      --interactive             allow interactive prompts for unspecified parameter assignments
  -l, --labels string           comma separated labels to apply to the trial
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

