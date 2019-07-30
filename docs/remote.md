# Remote Server Configuration

To configure a remote server (e.g. when using the [Enterprise solution](https://www.carbonrelay.com/red-sky-ops/)), you must provide connection parameters through the Red Sky Ops configuration.

## Connection Details

To connect to a remote server, you must provide an API endpoint to connect to the remote server (note that trailing the `/api` is required):

```sh
$ redskyctl config set address https://example.carbonrelay.dev/api
```

Additionally, you may need to specify OAuth2 credentials:

```sh
$ redskyctl config set oauth2.client_id XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
$ redskyctl config set oauth2.client_secret XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

## Applying Configuration

To apply the Red Sky Ops configuration to the current cluster, first view your existing configuration to verify it is correct:

```sh
$ redskyctl config
```

This will display the contents of your `~/.redsky` file plus any default values or environment variables that have been set.

Your configuration is applied (either created or updated) to the cluster through the `redskyctl init` command: if your configuration was valid when you last ran `init` there is no need to re-apply your configuration.

Once you have verified the configuration, you can apply it to the cluster:

```sh
$ redskyctl init
```

Alternately, you can store your configuration in `client-config` secret of the `redsky-system` namespace. You can also view this secret to verify the effective configuration values.
