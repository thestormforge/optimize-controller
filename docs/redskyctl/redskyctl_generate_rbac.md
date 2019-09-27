## redskyctl generate rbac

Generate experiment roles

### Synopsis

Generate an experiment manifest from a configuration file

```
redskyctl generate rbac [flags]
```

### Options

```
  -f, --filename string    File that contains the experiment to extract roles from.
  -h, --help               help for rbac
      --include-names      Include resource names in the generated role.
      --role-name string   Name of the cluster role to generate (default is to use a generated name).
```

### Options inherited from parent commands

```
      --address string      Absolute URL of the Red Sky API.
      --context string      The name of the kubeconfig context to use.
      --kubeconfig string   Path to the kubeconfig file to use for CLI requests.
  -n, --namespace string    If present, the namespace scope for this CLI request.
```

### SEE ALSO

* [redskyctl generate](redskyctl_generate.md)	 - Generate Red Sky Ops obejcts

