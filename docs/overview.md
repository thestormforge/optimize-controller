# Overview

## Architecture

Red Sky Ops is composed of three parts:

1. The Red Sky Ops Controller, which runs on your cluster,
2. `redskyctl`, a CLI tool for interacting with the manager, and
3. A Red Sky Ops Server (available as an enterprise product) that automatically generates suggested configurations for your application.

The Red Sky Ops Controller is a Kubernetes [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) written in Go. It is comprised of several Custom Resource Definitions (CRDs) and a manager that runs in your cluster in a dedicated namespace. The CRDs allow you to create Experiment and Trial objects in your cluster. The controller manages those experiments and trials, and communicates with the Red Sky Ops Server if configured to do so. The manager can be installed on your cluster using `redskyctl init` (see [Installation](install.md)).

The Red Sky Ops Tool `redskyctl` is a command-line tool for interacting with the controller. It can be used to install the manager, create trials by assigning parameters interactively, and more. It is meant to be used in combination with `kubectl`.

If you are on a Red Sky Ops Enterprise plan, you can configure your cluster to connect to a Red Sky Ops Server. The server produces experiment suggestions automatically.

## Concepts

### Experiments

An **Experiment** is the basic unit of organization in Red Sky Ops. The purpose of an experiment is to try configurations of an application and measure their impact. To accomplish this, experiments:

#### Parameters

A **Parameter** is the input to an experiment. A parameter has a minimum value, a maximum value, and a name. Currently, only integer parameters are supported. Taken together, an experiment's parameters define the **search space**: the total domain of all possible configurations. For in-depth explanation, see the [parameters](parameters.md) page.

#### Trials

A **Trial** is a single run of an experiment, with values assigned to every parameter. An experiment typically consists of many trials.

#### Metrics

A **Metric** is the output or outcome of a trial. They are used to measure the result of a particular choice of parameters. A metric is a numeric value that an experiment attempts to minimize (like cost in dollars) or maximize (like throughput) by adjusting the values of parameters. For in-depth explanation, see the [metrics](metrics.md) page.

### Putting it all together

* Create an **experiment** to deploy and test an application
* Make applications manifests configurable via **parameters**
* Run **trials** with specific values assigned to each parameter
* Assess the outcome of each trial by measuring one or more **metrics**

