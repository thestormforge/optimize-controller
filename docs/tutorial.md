# Advanced Tutorial

## Prerequisites

You must have a Kubernetes cluster. Additionally, you will need a local configured copy of `kubectl`. This example requires more resources then the [quick start](quickstart.md) tutorial, therefore you will need something larger then a typical minikube cluster. A three node cluster with 12 total vCPUs (4 on each node) and 24GB total memory (8GB on each node) is generally sufficient.

A local install of [Kustomize](https://github.com/kubernetes-sigs/kustomize/releases) (v3.0.0+) is required to manage the objects in you cluster.

Additionally, you will to initialize Red Sky Ops in your cluster. You can download a binary for your platform from the [releases page](https://github.com/redskyops/k8s-experiment/releases) and run `redskyctl init`. For more details, see [the installation guide](install.md).

## Example Resources

The resources for this tutorial can be found in the [`/examples/tutorial/`](https://github.com/redskyops/k8s-experiment/tree/master/examples/tutorial) directory of the `k8s-experiment` source repository.

`kustomization.yaml`
: The input to Kustomize used to build the Kubernetes object manifests for this example.

`service-account.yaml`
: This experiment will use Red Sky Ops "setup tasks". Setup tasks are a simplified way to apply bulk state changes to a cluster (i.e. installing and uninstalling an application or it's components) before and after a trial run. To use setup tasks, we will create a separate service account with additional privileges necessary to make these modifications.

`experiment.yaml`
: The actual experiment object manifest; this includes the definition of the experiment itself (in terms of assignable parameters and observable metrics) as well as the instructions for carrying out the experiment (in terms of patches and metric queries). Feel free to edit the parameter ranges and change the experiment name to avoid conflicting with other experiments in the cluster.

`config/`
: This directory contains manifests for additional cluster state required to run the experiment. For example, `config/prometheus.yaml` creates a minimal Prometheus deployment used to collect metrics during a trial run. The `config/logstash-values.yaml` are Helm values used to configure a release of Logstash from a trial setup task. Additional configuration for Filebeat (load generation) and other Prometheus exporters (use for cost estimates) are also present in the configuration directory.

## Experiment Lifecycle

Creating an Red Sky Ops experiment stores the experiment state in your cluster (if you are using the Enterprise solution the definition of the experiment is also synchronized to the server to begin searching for optimal assignments). No additional objects are created until trial assignments have been suggested.

Once assignments have been suggested, a trial run will start generating workloads for your cluster. The creation of a trial object populated with assignments will initiate the following work:

1. If the experiment contains setup tasks, a new job will be created for that work.
2. The patches defined in the experiment are applied to the cluster.
3. The status of all patched objects is monitored, the trial run will wait for them to stabilize.
4. The trial job specified in the experiment is created (the default behavior simply executes a timed sleep).
5. Upon completion of the trial job, metric values are collected.
6. If the experiment contains setup tasks, another job will be created to clean up the state created by the initial setup task job.

## Running the Experiment

From the `/examples/tutorial` directory, apply the required configuration to the cluster (note that the example kustomization leverages Kustomize features that may not be available from `kubectl apply -k`):

```sh
$ kustomize build | kubectl apply -f -
```

When configured to use the Enterprise solution, trials will be created automatically. You may interactively suggest trial assignments to start a trial run as well:

```sh
$ redskyctl suggest --name elk --interactive
```

## Monitoring the Experiment

Both `experiments` and `trials` are created as custom Kubernetes objects. You can see a summary of the objects using `kubectl get`; on compatible clusters, trial objects will also display their parameter assignments and (upon completion) observed values.

The experiment objects themselves will not have their state modified over the course of a trial run: once created they represent generally static state.

Trial objects will undergo a number of state progressions over the course of a trial run. These progressions can be monitored by watching the "status" portion of the trial object (e.g. when viewing `kubectl get trials -o yaml <NAME>`).

The trial object will also own several (one to three) job objects depending on the experiment; those jobs will be labeled using the trial name (e.g. `trial=<name>`) and are typically named using the trial name as a prefix. The `-create` and `-delete` suffixes on job names indicate setup tasks (also labeled with `role=trialSetup`).

## Re-running the Experiment

Once a trial run is complete, you can run additional trials using `redskyctl suggest` (again, this done automatically when using the Enterprise solution).

The tutorial experiment is not configured to isolate trials to individual namespaces: attempting to run a trial for the tutorial experiment while another tutorial experiment trial is in progress will cause conflicts and lead to inconsistent states.

If you need to upgrade Red Sky Ops between trial runs, you may need to reset Red Sky Ops: in this case you will need to re-apply the kustomization.
