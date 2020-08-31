## redskyctl suggest

Suggest assignments

### Synopsis

Suggest assignments for a new trial run

```
redskyctl suggest NAME [flags]
```

### Options

```
  -A, --assign stringToString   Assign an explicit value to a parameter. (default [])
      --default string          Select the behavior for default values; one of: none|min|max|rand.
  -h, --help                    help for suggest
      --interactive             Allow interactive prompts for unspecified parameter assignments.
  -l, --labels string           Comma separated labels to apply to the trial.
```

### Options inherited from parent commands

```
      --context string        The name of the redskyconfig context to use. NOT THE KUBE CONTEXT.
      --kubeconfig string     Path to the kubeconfig file to use for CLI requests.
  -n, --namespace string      If present, the namespace scope for this CLI request.
      --redskyconfig string   Path to the redskyconfig file to use.
```

### SEE ALSO

* [redskyctl](redskyctl.md)	 - Kubernetes Exploration

