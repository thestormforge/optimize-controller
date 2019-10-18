# Web App Example

## Introduction
A simple distributed application based on the official Docker [voting app](https://github.com/dockersamples/example-voting-app).
This application allows users to vote on "cats" vs. "dogs", and serves the results in a simple web page.

From the Docker example-voting-app documentation:
The application architecture is made of 5 components (see diagram below):
* A front-end web app in [Python](/vote) or [ASP.NET Core](/vote/dotnet) which lets you vote between two options
* A [Redis](https://hub.docker.com/_/redis/) or [NATS](https://hub.docker.com/_/nats/) queue which collects new votes
* A [.NET Core](/worker/src/Worker), [Java](/worker/src/main) or [.NET Core 2.1](/worker/dotnet) worker which consumes votes and stores them in…
* A [Postgres](https://hub.docker.com/_/postgres/) or [TiDB](https://hub.docker.com/r/dockersamples/tidb/tags/) database backed by a Docker volume
* A [Node.js](/result) or [ASP.NET Core SignalR](/result/dotnet) webapp which shows the results of the voting in real time

![Architecture diagram](architecture.png)

In this example, we demonstrate how to tune a typical application with several components, using [Locust](https://locust.io/). Locust makes repeated HTTP calls to the voting service, thereby simulating user load on the application.

## Prerequisites

You must have a Kubernetes cluster. We recommend using a cluster with 4 nodes, 16 vCPUs (4 on each node) and 32GB of memory (8 on each node). Additionally, you will need a local configured copy of `kubectl`.

A local install of [Kustomize](https://github.com/kubernetes-sigs/kustomize/releases) (v3.1.0+) is required to manage the objects in you cluster.

Additionally, you will to initialize Red Sky Ops in your cluster. You can download a binary for your platform from the [releases page](https://github.com/redskyops/k8s-experiment/releases) and run `redskyctl init` (while connected to your cluster). For more details, see [the installation guide](https://github.com/redskyops/k8s-experiment/blob/master/docs/install.md).

## Example Resources

The resources for this tutorial can be found in the [`/examples/voting-web-app/`](https://github.com/redskyops/k8s-experiment/tree/master/examples/voting-web-app) directory of the `k8s-experiment` source repository.

`experiment.yaml`
: The actual experiment object manifest; this includes the definition of the experiment itself (in terms of assignable parameters and observable metrics) and the instructions for carrying out the experiment (in terms of load generation and metric queries). Here is where you may change parameter ranges and the experiment name (to avoid experiment name conflicts in the cluster).

`kustomization.yaml`
: The input to Kustomize used to build the Kubernetes object manifests for this example.

`locust-config.yaml`
: Configuration for the Locust load test.

`postgres.yaml`
: The manifest for the PostgreSQL database.

`redis.yaml`
: The manifest for the Redis queue.

`results-exporter.yaml`
: The manifest for the results exporter.

`results-service.yaml`
: The manifest for the results service.

`voting-service.yaml`
: The manifest for the voting service.

`worker.yaml`
: The manifest for the .NET worker.

`exporter/`
: This directory contains files required to run the results exporter. The exporter's role is to expose the throughput metric over HTTP (in contrast to the original websocket implementation in the results service) so that it can be collected by the jsonpath metric collector.
`app.py` contains the exporter application, `requirements.txt` are the python library requirements and `Dockerfile` contains the Docker commands to run the exporter container. There is no need to build the image; the folder is only included for reference.

More information can be found on the [docker example voting app](https://github.com/dockersamples/example-voting-app) and [Locust](https://docs.locust.io/en/stable/) docs.

## Experiment Lifecycle

For every trial, several pods will come up, corresponding to the five services of the application plus the results exporter:

1. `voting-service`
2. `redis`
3. `worker`
4. `db`
5. `results-service`
6. `results-exporter`

For more information on running, monitoring and maintaining experiments, please refer to our [quickstart](https://github.com/redskyops/k8s-experiment/blob/master/docs/quickstart.md) and [experiment lifecycle](https://github.com/gramLabs/k8s-experiment/blob/master/docs/lifecycle.md) documentation.
