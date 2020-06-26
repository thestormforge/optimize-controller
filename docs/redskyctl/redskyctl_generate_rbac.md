## redskyctl generate rbac

Generate experiment roles

### Synopsis

Generate RBAC manifests from an experiment manifest

```
redskyctl generate rbac [flags]
```

### Options

```
      --cluster-role           Generate a cluster role. (default true)
      --cluster-role-binding   When generating a cluster role, also generate a cluster role binding. (default true)
  -f, --filename string        File that contains the experiment to extract roles from.
  -h, --help                   help for rbac
      --include-names          Include resource names in the generated role.
  -o, --output format          Output format. One of: json|yaml (default "yaml")
      --role-name string       Name of the cluster role to generate (default is to use a generated name).
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

