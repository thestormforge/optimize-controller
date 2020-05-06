# Server Configuration

To configure a remote server (e.g. when using the [Enterprise solution](https://www.carbonrelay.com/red-sky-ops/)), you must provide connection parameters through the Red Sky Ops configuration.

## Connection Details

To authorize the Red Sky Manager running in your cluster, you must first prepare the necessary credentials on your workstation (i.e. where you run the `redskyctl` program from). This can be done using a web-based form:

```sh
redskyctl login
```

Alternatively, you can get a one-time use code to enter into a browser from another device:

```sh
redskyctl login --url
```

## Applying Configuration

To apply the Red Sky Ops configuration to the current cluster, first view your existing configuration to verify it is correct:

```sh
redskyctl config view
```

This will display the contents of your `~/.config/redsky/config` file plus any default values or environment variables that have been set.

Your configuration is applied (either created or updated) to the cluster through the `redskyctl authorize-cluster` command. Additionally, the `redskyctl init` command will automatically perform authorization if your connection details are available. If your configuration was valid when you last ran `init`, there is no need to re-apply your configuration.

Once you have verified the configuration, you can ensure your Red Sky Manager deployment is up-to-date:

```sh
redskyctl init
```

Alternately, you can store your configuration in the `redsky-manager` secret of the `redsky-system` namespace. You can also view this secret to verify the effective configuration values.

## Helm Values

If you are installing the Red Sky Ops Controller in your cluster using Helm, you can run `redskyctl authorize-cluster --helm-values` to produce a `values.yaml` file with the necessary extra configuration.
