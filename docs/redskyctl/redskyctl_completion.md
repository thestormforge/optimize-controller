## redskyctl completion

Output shell completion code

### Synopsis

Output shell completion code which can be evaluated to provide interactive completion of commands.

```
redskyctl completion SHELL [flags]
```

### Examples

```
# Load the completion code for zsh into the current shell
source <(redskyctl completion zsh)
# Set the completion code for zsh to autoload (assuming '$ZSH/completions' is part of 'fpath')
redskyctl completion zsh > $ZSH/completions/_redskyctl
```

### Options

```
  -h, --help   help for completion
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

