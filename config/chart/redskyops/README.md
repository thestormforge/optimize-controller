# Red Sky Ops

## Chart Repository

The Red Sky Ops chart repository can be configured in Helm as follows:

```sh
helm repo add redsky https://redskyops.dev/charts/
helm repo update
```

## Installing the Chart

The Red Sky Ops manager can be installed using the Helm command:

```sh
helm install redsky redsky/redskyops --namespace redsky-system
```

The recommended namespace (`redsky-system`) and release name (`redsky`) are consistent with an install performed using the `redskyctl` tool (see the [install guide](https://redskyops.dev/docs/install/) for more information).

## Configuration

The following configuration options are available:

| Parameter                   | Description                                               |
| --------------------------- | --------------------------------------------------------- |
| `redskyImage`               | Docker image name                                         |
| `redskyTag`                 | Docker image tag                                          |
| `redskyImagePullPolicy`     | Pull policy for the Docker image                          |
| `remoteServer.enabled`      | Flag indicating that the remote server should be used     |
| `remoteServer.clientID`     | OAuth2 client identifier                                  |
| `remoteServer.clientSecret` | OAuth2 client secret                                      |
| `rbac.create`               | Specify whether RBAC resources should be created          |
| `rbac.bootstrapPermissions` | Flag indicating default permissions should be included    |
| `rbac.extraPermissions`     | Flag indicating additional permissions should be included |
