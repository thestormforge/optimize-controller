## redskyctl generate rbac

Generate experiment roles

### Synopsis

Generate RBAC manifests from an experiment manifest

```
redskyctl generate rbac [flags]
```

### Options

```
      --cluster-role           generate a cluster role (default true)
      --cluster-role-binding   when generating a cluster role, also generate a cluster role binding (default true)
  -f, --filename string        file that contains the experiment to extract roles from
  -h, --help                   help for rbac
      --include-names          include resource names in the generated role
  -o, --output format          output format. one of: json|yaml (default "yaml")
      --role-name string       name of the cluster role to generate (default is to use a generated name)
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

