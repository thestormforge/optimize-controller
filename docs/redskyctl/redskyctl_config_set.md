## redskyctl config set

Modify the configuration file

### Synopsis

Modify the Red Sky Ops configuration file

```
redskyctl config set NAME [VALUE] [flags]
```

### Examples

```
Names are: address, oauth2.token, oauth2.token_url, oauth2.client_id, oauth2.client_secret

# Set the remote server address
redskyctl config set address http://example.carbonrelay.io
```

### Options

```
  -h, --help   help for set
```

### Options inherited from parent commands

```
      --address string      Absolute URL of the Red Sky API.
      --context string      The name of the kubeconfig context to use.
      --kubeconfig string   Path to the kubeconfig file to use for CLI requests.
  -n, --namespace string    If present, the namespace scope for this CLI request.
```

### SEE ALSO

* [redskyctl config](redskyctl_config.md)	 - Work with the configuration file

