# Quick Start

This is brief guide to get you up and running with Red Sky Ops as quickly as possible.

## Prerequisites

You must have a Kubernetes cluster. Additionally, you will need a local configured copy of `kubectl`. The Red Sky Ops Tool will use the same configuration as `kubectl` (usually `$HOME/.kube/config`) to connect to your cluster.

A local install of [Kustomize](https://github.com/kubernetes-sigs/kustomize/releases) (v3.1.0+) is required to build the objects for your cluster.

If you are planning to create the postgres experiment from this guide, a [minikube](https://kubernetes.io/docs/setup/learning-environment/minikube/) cluster will be sufficient.

## Install the Red Sky Ops Tool

[Download](https://github.com/redskyops/redskyops-controller/releases) and install the `redskyctl` binary for your platform. You will need to rename the downloaded file and mark it as executable.

For more details, see [the installation guide](install.md).

## Initialize the Red Sky Ops Manager

Once you have the Red Sky Ops Tool you can initialize the manager in your cluster:

```sh
$ redskyctl init
```

## Create a Simple Experiment

Generally you will want to write your own experiments to run trials on your own applications. For the purposes of this guide we can use the postgres example found in the `redskyops-recipes` [repository on GitHub](https://github.com/redskyops/redskyops-recipes/tree/master/postgres):

```sh
$ kustomize build github.com/redskyops/redskyops-recipes/postgres | kubectl apply -f -
```

## Run a Trial

With your experiment created, you can be begin running trials by suggesting parameter assignments locally. Each trial will create one or more Kubernetes jobs and will conclude by collecting a small number of metric values indicative of the performance for the trial.

```sh
$ redskyctl generate trial --assign memory=1000 --assign cpu=500 -f <(kubectl get experiment postgres-example -o yaml)  | kubectl create -f -
```

Or alternatively, To interactively create a new trial for the example experiment, run the following.
You will be prompted to enter a value for each parameter in the experiment and a new trial will be created.

```sh
$ redskyctl generate trial --interactive -f <(kubectl get experiment postgres-example -o yaml)
```

You can monitor the progress using `kubectl`:

```sh
$ kubectl get trials
```

When running interactive trials in a single namespace, be sure only trial is active at a time.

After the trial is complete, you will be able to view the parameters and the metrics generated from the trial. The metrics can be used to gauge how effective the used parameters were.

## Removing the Experiment

To clean up the data from your experiment, simply delete the experiment. The delete will cascade to the associated trials and other Kubernetes objects:

```sh
$ kubectl delete experiment postgres-example
```

## Next Steps

Congratulations! You just ran your first experiment. You can move on to a more [advanced tutorial](tutorial.md) or browse the rest of the documentation to learn more about the Red Sky Ops Kubernetes experimentation product.
