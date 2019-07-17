# Red Sky OPS - ELK tutorial

## Prerequisites

Ensure you have the latest version of [Kustomize](https://github.com/kubernetes-sigs/kustomize/releases) installed (currently version 3.0.2).

Make sure you are connected to the cluster of your choice (We use a small cluster on GKE for this tutorial) and have [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) installed.

Download the latest `redskyctl` binary [GitHub](https://github.com/redskyops/k8s-experiment/releases) to your /usr/local/bin folder.


## Setting up the Experiment

In examples/tutorial you can see the experiment.yaml for this experiment, as well as the /config that's necessary for some ELK components and metric logging.
Feel free to edit the parameter ranges in the experiment.yaml, and change the experiment name (make sure to avoid conflicts if you are running multiple experiments). You can also use this file as a template for a custom experiment for your specific application.


## Running the Experiment

First, we need to install redskyctl in the cluster:

```
$ redskyctl init
```

From the /examples/tutorial folder, apply the required configuration to the cluster:

```
$ kustomize build | kubectl apply -f -
```

To start the experiment, apply it to the cluster:

```
$ kubectl apply -f experiment.yaml
```

In the Enterprise version of Red Sky, `redskyctl` will start creating trials automatically.
To manually suggest trial configurations use:

```
$ redskyctl suggest --name elk --interactive
```

and `redskyctl` will launch your trial.


## Monitoring the Experiment

Both `trials` and `experiments` are created as kube objects.
You can see the assignments and results of your trials using:

```
$ kubectl get trials
```

And list your experiments using:

```
$ kubectl get experiments
```

You can also view their respective yamls by applying the -o yaml flag as usual.

More experiment monitoring options will be coming shortly!


## Re-running the Experiment

If necessary, download a updated version of `redskyctl` and re-run `redskyctl init` to update the Red Sky manager to the latest version. Use `redskyctl version` to check what version you are currently using.

NOTE: Pre-release versions of `redskyctl` _MAY_ require you to run `redskyctl reset` prior to `init`, this is only required when the CRD changes or when you want to purge all experiment and trial data from your cluster.

Re-apply the experiment configuration and the experiment itself:

```
$ kustomize build | kubectl apply -f -
$ kubectl apply -f experiment.yaml
```
