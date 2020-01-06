# Server Configuration

To configure a remote server (e.g. when using the [Enterprise solution](https://www.carbonrelay.com/red-sky-ops/)), you must provide connection parameters through the Red Sky Ops configuration.

## Connection Details

To authorize the Red Sky Manager running in your cluster, you must first prepare the necessary credentials on your workstation (i.e. where you run the `redskyctl` program from). This can be done using a web-based form:

```sh
redskyctl login
```

Alternatively, you can set the credentials manually:

```sh
redskyctl config set oauth2.client_id XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.XXX.XXX.XXX
redskyctl config set oauth2.client_secret XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

## Applying Configuration

To apply the Red Sky Ops configuration to the current cluster, first view your existing configuration to verify it is correct:

```sh
redskyctl config
```

This will display the contents of your `~/.redsky` file plus any default values or environment variables that have been set.

Your configuration is applied (either created or updated) to the cluster through the `redskyctl authorize` command. Additionally, the `redskyctl init` command will automatically perform authorization if your connection details are available. If your configuration was valid when you last ran `init`, there is no need to re-apply your configuration.

Once you have verified the configuration, you can ensure your Red Sky Manager deployment is up-to-date:

```sh
redskyctl init
```

Alternately, you can store your configuration in the `redsky-manager` secret of the `redsky-system` namespace. You can also view this secret to verify the effective configuration values.
