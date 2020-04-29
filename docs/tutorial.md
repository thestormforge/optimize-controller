# Tutorial

## Prerequisites

* Kubernetes Cluster
* `kubectl` properly configured
* [redskyctl](https://github.com/redskyops/redskyops-controller/releases)
* Red Sky Ops controller running ( `redskyctl init` )

This example will deploy Elasticsearch and requires more resources then the [quick start](quickstart.md) example, therefore you will need something larger then a typical minikube cluster.
A four node cluster with 32 total vCPUs (8 on each node) and 64GB total memory (16GB on each node) is generally sufficient.

## Experiment Lifecycle

Creating a Red Sky Ops experiment stores the experiment state in your cluster.
If using the Enterprise solution, the experiment definition is also synchronized to our machine learning server.
No additional objects are created until trial assignments have been suggested (either manually or using our machine learning API, see next section on adding manual trials).

Once assignments have been suggested, a trial run will start generating workloads for your cluster.
The creation of a trial object populated with assignments will initiate the following work:

1. If the experiment contains setup tasks, a new job will be created for that work.
2. The patches defined in the experiment are applied to the cluster.
3. The status of all patched objects is monitored, the trial run will wait for them to stabilize.
4. The trial job specified in the experiment is created (the default behavior simply executes a timed sleep).
5. Upon completion of the trial job, metric values are collected.
6. If the experiment contains setup tasks, another job will be created to clean up the state created by the initial setup task job.

## Tutorial Manifests

The manifests for this tutorial can be found in the [`elasticsearch`](https://github.com/redskyops/redskyops-recipes/tree/master/elasticsearch) directory of the [`redskyops-recipes`](https://github.com/redskyops/redskyops-recipes) repository.

`service-account.yaml`
: This experiment will use Red Sky Ops "setup tasks". Setup tasks are a simplified way to apply bulk state changes to a cluster (i.e. installing and uninstalling an application or it's components) before and after a trial run. To use setup tasks, we will create a separate service account with additional privileges necessary to make these modifications.

`experiment.yaml`
: The actual experiment object manifest; this includes the definition of the experiment itself (in terms of assignable parameters and observable metrics) as well as the instructions for carrying out the experiment (in terms of patches and metric queries). Feel free to edit the parameter ranges and change the experiment name to avoid conflicting with other experiments in the cluster.

`rally-config.yaml`
: This experiment makes use of [rally](https://github.com/elastic/rally) to test Elasticsearch. This contains the configuration for rally.

## Running the Experiment

We'll need to apply the manifests listed above for our experiment.

```sh
$ kubectl apply -f https://raw.githubusercontent.com/redskyops/redskyops-recipes/master/elasticsearch/service-account.yaml
serviceaccount/redsky created
clusterrolebinding.rbac.authorization.k8s.io/redsky-cluster-admin created

$ kubectl apply -f https://raw.githubusercontent.com/redskyops/redskyops-recipes/master/elasticsearch/rally-config.yaml
configmap/rally-ini created

$ kubectl apply -f https://raw.githubusercontent.com/redskyops/redskyops-recipes/master/elasticsearch/experiment.yaml
experiment.redskyops.dev/elasticsearch-example created
```

Verify all resources are present:

```sh
$ kubectl get experiment,sa,cm
NAME                                             STATUS
experiment.redskyops.dev/elasticsearch-example   Never run

NAME                     SECRETS   AGE
serviceaccount/default   1         4h7m
serviceaccount/redsky    1         36s

NAME                  DATA   AGE
configmap/rally-ini   1      23s
```

Next we'll need to create a trial for our experiment.
When configured to use the Enterprise solution, trials will be created automatically.
In this example, we'll use some predefined values for the trial, however you may interactively suggest trial assignments to start a trial run as well via `redskyctl generate trial --interactive -f <(kubectl get experiment elasticsearch-example -o yaml)`.

```sh
$ redskyctl generate trial \
    --assign data_memory=2000 \
    --assign data_cpu=500 \
    --assign data_replicas=1 \
    --assign data_heap_percent=20 \
    --assign client_memory=2000 \
    --assign client_cpu=500 \
    --assign client_replicas=1 \
    --assign client_heap_percent=20 \
    -f <(kubectl get experiment elasticsearch-example -o yaml) | \
  kubectl create -f -
trial.redskyops.dev/elasticsearch-example-5bfmt created
```

Now you can view the trial status:

```sh
$ kubectl get trial -l redskyops.dev/experiment=elasticsearch-example
NAME                          STATUS       ASSIGNMENTS                                                                                                                                            VALUES
elasticsearch-example-gvwwl   Setting up   data_memory=2000, data_cpu=500, data_replicas=1, data_heap_percent=20, client_memory=2000, client_cpu=500, client_replicas=1, client_heap_percent=20
```

## Monitoring the Experiment

Both `experiments` and `trials` are created as custom Kubernetes objects.
You can see a summary of the objects using `kubectl get trials,experiments`; on compatible clusters, trial objects will also display their parameter assignments and (upon completion) observed values.

The experiment objects themselves will not have their state modified over the course of a trial run: once created they represent generally static state.

Trial objects will undergo a number of state progressions over the course of a trial run.
These progressions can be monitored by watching the "status" portion of the trial object (e.g. when viewing `kubectl get trials -o yaml <TRIAL NAME>`).

The trial object will also own several (one to three) job objects depending on the experiment; those jobs will be labeled using the trial name (e.g. `trial=<name>`) and are typically named using the trial name as a prefix.
The `-create` and `-delete` suffixes on job names indicate setup tasks (also labeled with `role=trialSetup`).

## Collecting Experiment Output

Once an experiment is underway and some trials have completed, you can get the trial results using `kubectl`:

```sh
$ kubectl get trials -l redskyops.dev/experiment=elasticsearch-example
```

## Re-running the Experiment

Once a trial run is complete, you can run additional trials using `redskyctl generate trial` or if using the Enterprise solution, a new trial will be generated automatically.

The tutorial experiment is not configured to isolate trials to individual namespaces: attempting to run a trial for the tutorial experiment while another tutorial experiment trial is in progress will cause conflicts and lead to inconsistent states.
