# Cordelia
The Kubernetes Experimentation Platform

## Introduction

Cordelia is a platform for running parameterized experiments on Kubernetes clusters.

## How It Works

Cordelia is an experiment controller running inside a Kubernetes cluster. To start an experiment, you must first define the tunable state of the cluster along with the metrics that will be used for outcome evaluation. Upon creation of a `Trial` resource, the controller starts a new run by using parameter suggestions obtained by the server specified in an `Experiment` resource. Over the course of the trial, additional suggestions may be introduced and are run on the cluster. Each run concludes by capturing metrics prescribed by the experiment. A trial will continue to perform runs indefinitely as long suggestions continue to be provided by the server.

### Defining An Experiment

The search space of an experiment is defined using a set of target resource selectors and corresponding patch templates. The templates are executed against a parameter context consisting of JSON primitive values.

The outcome of an experiment trial run is defined using a set of metrics. Metrics are represented in the form of PromQL queries to be evaluated against a Prometheus instance at the conclusion of a run.

A parameter suggestion service must be configured to supply trial instances with context values.

### Definition A Trial

A trial may reference an experiment to obtain patches, metric queries and the suggestion service configuration.

A job template is used to create a Kubernetes job representing the trial run. Each trial run will create a new job from the template and the completion of the job will trigger collection of the metrics. If the application being tested is already under load, the trial run job can simply be a timed sleep.

## Running An Experiment

An experiment is run on a Kubernetes cluster which has the Cordelia controller installed. To start the experiment, create a new `Trial` resource; the controller will use the `Trial` creation to start polling for available suggestions. When a suggestion is available, it will apply the desired state changes to the cluster before creating the trial run job. At the conclusion of the trial run, the metric values are captured.

An experiment can also be run in parallel, either using namespaces within a cluster or by leveraging the resources of multiple clusters. In both cases, the name of the `Trial` resource is used to coordinate controllers polling suggestions from the same trial. The suggestion source must be capable of providing multiple simultaneous suggestions or controllers will be starved of work.

### Mutating State

The state of the cluster is modified through patches applied to resources matched through configured selectors. In addition to patching existing objects, Cordelia will monitor the creation of matching resources and will patch them on admission to the Kubernetes API. Resources patched during admission need not be patched later in the run (such an operation would effectively do nothing).
