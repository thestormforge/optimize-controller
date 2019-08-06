# Installing Red Sky Ops

There are two parts to Red Sky Ops: the `redskyctl` tool and Red Sky Ops Manager (running in your cluster).

## Installing the Red Sky Ops Tool

### Binary Releases

You can download binaries directly from the [releases page](https://github.com/redskyops/k8s-experiment/releases).

### Using cURL and jq

To download the latest release, select your platform (`linux` or `darwin`) and run:

```sh
os=linux # Or 'darwin'
curl -s https://api.github.com/repos/redskyops/k8s-experiment/releases/latest |\
  jq -r ".assets[] | select(.name | contains(\"${os:-linux}\")) | .browser_download_url" |\
  xargs curl -L -o redskyctl
chmod +x redskyctl
sudo mv redskyctl /usr/local/bin/
```

## Installing the Red Sky Ops Manager

The Red Sky Ops Manager runs inside your Kubernetes cluster. It can be configured to talk to an Enterprise server for improved capabilities.

### Easy Install

To perform an easy install, simply run `redskyctl init`. This will create a new `redsky-system` namespace and will create a Kubernetes job to manage the actual installation.

Using `redskyctl init` is safe for multiple invocations; in fact re-running it with a new version of `redskyctl` is also the easiest way to upgrade your in cluster components.

### Easy Enterprise Install

If you are subscribing to the Enterprise product, please contact your sales representative for additional configuration prior to running `redskyctl init`. If you just want to get started, you can always apply the additional configuration later.

### Advanced Installation

If you have specific security requirements or the default RBAC configuration for the easy install is too permissive for your environment, there are a number of ways to obtain the raw Red Sky Ops Manager manifests:

1. Using `redskyctl init --bootstrap` will create a paused Kubernetes job, you can adjust the bootstrap configuration in cluster and proceed with the installation by scaling the job to 1.
2. Using `redskyctl init --dry-run` will print the raw manifests used during installation, however this still requires creating a Kubernetes job. This option can be combined with the `--bootstrap` option to get the raw manifests of the bootstrap job.
3. Using Docker to run the `setuptools` image directly. For example, `docker container run --rm $(redskyctl version --setuptools)` will produce the same output as `redskyctl init --dry-run` without requiring a configured Kubernetes context.

## Upgrading the Red Sky Ops Manager

The preferred way to upgrade the Red Sky Ops Manager is to install the latest version of `redskyctl` locally and run `redskyctl config fix` before re-running `redskyctl init`. Use `redskyctl version` to check the current version numbers.

In some cases there may be incompatibilities between versions requiring an uninstall prior to the installation of the new version: please consult the release notes for the version you are installing.

## Uninstalling the Red Sky Ops Manager

To remove the Red Sky Ops Manager completely from your cluster, run `redskyctl reset`.

*IMPORTANT* Running the reset command will also remove all of the Red Sky Ops data. Ensure you have backed up any information in the cluster prior to running this command.
