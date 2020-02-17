## redskyctl generate rbac

Generate experiment roles

### Synopsis

Generate an experiment manifest from a configuration file

```
redskyctl generate rbac [flags]
```

### Options

```
      --bootstrap-role      Generate the default cluster used for initial installations
      --extra-permissions   Generate permissions required for features like namespace creation
  -f, --filename string     File that contains the experiment to extract roles from.
  -h, --help                help for rbac
      --include-names       Include resource names in the generated role.
      --role-name string    Name of the cluster role to generate (default is to use a generated name).
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

