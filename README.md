# Red Sky Ops - Kubernetes Experiments

[![CircleCI](https://circleci.com/gh/redskyops/k8s-experiment.svg?style=svg)](https://circleci.com/gh/redskyops/k8s-experiment)

The Kubernetes Experiments project (k8s-experiment) supports the creation and execution of experiments used for the validation of configuration state through a series of trials.

## Installation

Downloads of the Red Sky Ops Tool can be found on the [release page](https://github.com/redskyops/k8s-experiment/releases). Download the appropriate binary for your platform and add it to your PATH.

To install the custom Kubernetes resources to you currently configured cluster, execute the `redskyctl init` command. To uninstall and remove all of the Red Sky Opts data, execute `redskyctl reset`.

## Getting Started

See the [tutorials](https://github.com/redskyops/k8s-experiment/blob/master/docs/tutorial.md).

An experiment modifies the state of the cluster using patches (e.g. strategic merge patches) represented as Go templates with parameter assignments for input. Metrics are typically collected using PromQL queries against an in-cluster Prometheus service. Optionally, setup tasks can be run before and after each trial: these tasks create or delete Kustomizations.

### Parameters

Parameters are named integers assigned from an inclusive range.

Note: when working with Kubernetes "quantity" values, you must use the integer notation (e.g. a CPU limit of "4.0" must be expressed as "4000m").

### Metrics

Metrics are named floating point values collected at the conclusion of a trial run.

Note: when using Prometheus metrics, PromQL queries must evaluate to a scalar value.

### Patches

Patches are Go Templates evaluated against parameter assignments that produce a patch supported by the Kubernetes API Server (e.g. strategic merge patches). Parameters are exposed via a `Values` map (e.g. `{{ .Values.x }}` would evaluate to the assignment of parameter "x").

### Setup Tasks

Setup tasks can be executed before or after a trial run. Each setup task builds a Kustomization and creates (prior to the trial run) or deletes (after the trial run) the resulting manifests.

Setup tasks can reference a Helm chart which will be fetched and evaluated locally as a resource in the Kustomization. Helm values can be assigned using the same Go Templates as patches.

## Development

This project was bootstrapped by [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) and inherits many of the stock conventions. Some notable exceptions are the inclusion of the `make tool` target for building the Red Sky Control tool and overloading `make docker-build` to produce both the Red Sky Experiment Manager image and the Setup Tools image.

To run the Red Sky Experiment Manager locally: first run `make install` to add the necessary Custom Resource Definitions (CRD) to you currently configured cluster; then run `make run` to start a local process (inheriting Red Sky Client API configuration from your current environment).

To run in cluster, first build the Docker images using `make docker-build`. Next, push the images to your cluster (if you are using minikube, just run `eval $(minikube docker-env)` prior to building the images): for example, `export IMG=us.gcr.io/$PROJECT_ID/k8s-experiment:$TAG` and `export SETUPTOOLS_IMG=us.gcr.io/$PROJECT_ID/setuptools:$TAG` before running `make docker-push` (choose your own unique value for `TAG`). Finally, run `make deploy` to apply the manifests, or use `make tool` and `bin/redskyctl-$GOOS-$GOARCH init`. 
