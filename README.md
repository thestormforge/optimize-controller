# StormForge Optimize - Controller

![Master](https://github.com/thestormforge/optimize-controller/workflows/Master/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/thestormforge/optimize-controller)](https://goreportcard.com/report/github.com/thestormforge/optimize-controller)


## Getting Started

Please refer to the documentation for detailed installation instructions: [Installing StormForge Optimize](https://docs.stormforge.io/getting-started/install/).

You will also find a [quick start](https://docs.stormforge.io/getting-started/quickstart/) guide and additional information about using StormForge Optimize.


## Development

This project was bootstrapped by [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) and inherits many of the stock conventions.

To run locally: first run `make install` to add the necessary Custom Resource Definitions (CRDs) to you currently configured cluster.

If you would like to start a local process (inheriting Kubeconfig and StormForge configuration from your current environment), first ensure that any manager in the cluster is disabled:

```sh
kubectl scale deployment optimize-controller-manager -n stormforge-system --replicas 0
make run
```

You can also debug using existing images (e.g. the latest CI builds): configure your debugger to pass the following arguments to the Go tools: `-ldflags "-X github.com/thestormforge/optimize-controller/v2/internal/setup.Image=ghcr.io/thestormforge/setuptools:edge -X github.com/thestormforge/optimize-controller/v2/internal/setup.ImagePullPolicy=Always"`.

Alternatively, if you would like create an image and run it in minikube, build the Docker images directly to the minikube host:

```sh
eval $(minikube docker-env)
make docker-build
make deploy
```

To run in a GKE cluster, you will need to push the images to GCR:

```sh
export PROJECT_ID=<GCP project ID where your cluster is running>
export TAG=<something unique>
export IMG=us.gcr.io/$PROJECT_ID/optimize-controller:$TAG
export SETUPTOOLS_IMG=us.gcr.io/$PROJECT_ID/setuptools:$TAG
export REDSKYCTL_IMG=us.gcr.io/$PROJECT_ID/redskyctl:$TAG
make docker-build
make docker-push
make deploy
```

You can also use `make tool` and `bin/redskyctl-$GOOS-$GOARCH init` in place of `make deploy` to use the actual versioned manifests used by the product.
