
# Introduction
- What is it
	- One-liner
		- `redsky-experiment` lets you create and run experiments on your kubernetes cluster. Red Sky Ops Enterprise uses k8s-experiment to vertically scale
- Background
	- Overview of Kube's resource limits/requests
		- https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
		- Kubernetes allows resource requests and limits to be set on containers and pods. This becomes the deployable unit
	- Autoscaling capabilities & limits
		- https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/
		- Algorithm details: https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details
		- Only works within existing requests/limits?
		- Allows scaling, but doesn't prevent overutilization per pod?
			- E.g. like it says, scales horizontally, but you're in charge of correctly scaling vertically
		- ^ Particularly for nominal usage. Helps with bursty demand, but doesn't optimize the base case (setting the correct requests/limits)
		- Doesn't tune application-level parameters like -Xmx, shards... anything you want!
			- Take a look at custom (extended?) resources? Could this be done (https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#extended-resources)
				- Honestly not sure what this is for
	- Benefits of application-level tuning (not just CPU, memory, replicas)
		- Deeper and broader reach
	- Automated, covering search space more effectively than a person could
		- Note to self: Careful not to mix enterprise offering with OSS offering
- Outline of features
	- Experiments, under conditions you define
		- Real load, load tests...
	- Other uses of the tool besides resource tuning
		- General experimentation on clusters
		- Call to Action: "Using RSO in a novel way? Tell us about it!" link to github, twitter?

## Draft
*Style Notes:*
- Present tense
- "Red Sky Ops" or `k8s-experiment`? If both, add a note about them being used interchangeably to the Introduction.
- Look into how often the k8s docs are broken up with headers, lists, etc

# Overview
## Introduction
**k8s-experiment** is a tool for experimenting with application configurations in Kubernetes. With `k8s-experiment` installed on your cluster, you can run experiments that manipulate resources and measure the outcome via one or more metrics.

When used as part of Red Sky Ops, `k8s-experiment` tunes your Kubernetes applications automatically using machine learning. Without the enterprise server, `k8s-experiment` can be used to manually experiment on your cluster, or— using `redskyctl`— can be integrated into existing automation workflows.

Although `k8s-experiment` was developed as part of Red Sky Ops to tune performance, it can run non-performance-related experiments on your cluster as well. This documentation describes the concepts and capabilities of `k8s-experiment` generically, and typically includes performance-related examples for relevant concepts.

*Using RSO in a novel way? Encounter an issue with these docs? We'd love to hear from you! File an issue on [GitHub](https://github.com/redskyops/k8s-experiment)" or reach out on [Twitter](https://twitter.com/redskyops1)*

## Motivation: Managing Resources in Kubernetes
This page borrows concepts and terminology from the [Kubernetes Concepts documentation](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#extended-resources).

Kubernetes has several built-in mechanisms for managing container compute resources. To begin, Pods may specify CPU and memory requests and limits for each of their containers.

Once containers have these limits set, Kubernetes can leverage them in several ways:

- The Kubernetes scheduler uses the resource requests to efficiently schedule pods onto nodes based on resource needs and availability ("bin packing")
- If pods exceed their CPU or memory limits, Kubernetes may evict them from their host node
- When used in combination with Quality of Service (QoS) classes or pod priority, Kubernetes can go a step further, choosing which pods to schedule and evict based on the quality of service they require
- To accommodate demand on pods, Kubernetes can "horizontally" scale them out by adding more replicas via Horizontal Pod Autoscaling (HPA). The HPA controller examines the CPU resource request and calculates utilization as a fraction of the requested amount to determine when to scale. If utilization is higher or lower than the target, it adds or removes pods.

While powerful, these tools are missing some aspects of resources untuned:
- Developers must devise CPU and memory requests and limits themselves, perhaps based on performance analysis locally or in a dev environment. (Scaling resources rather than replicas is sometimes referred to as Vertical Pod Scaling)
- Kubernetes cannot adjust application-specific settings that may have significant impact on performance from workload to workload. For instance: JVM heap size, shard size in Elasticsearch, or working memory on a self-hosted database would not be exposed through a consistent interface in Kubernetes, even if they may be set by environment variables or through a ConfigMap.

Red Sky Ops was developed to address these shortcomings. **k8s-experiment**  the tools needed to find the ideal configuration for a Kubernetes application. It support flexibly parameterizing application yaml; running trials with specific values filled into those yaml documents; taking measurements to determine the efficacy of trials; and grouping those trials into overarching Experiments.

For an in-depth explanation of the tool, proceed to the [Concepts]() section.

# Concepts (hm, duplicated below)
## Architecture
`k8s-experiment` is composed of three parts:
1. The manager, which runs on your cluster,
2. `redskyctl`, a CLI tool for interacting with the manager, and
3. An optional server (currently provided as an enterprise product) that automatically generates suggested configurations for your application.
<!-- TODO: Need a better name for the server. Also, confirm name of manager? And namespace? -->

The manager is a Kubernetes [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) written in Go. It is comprised of several Custom Resource Definitions (CRDs) and a Controller that runs in your cluster in a dedicated namespace. The CRDs allow you to create Experiment and Trial objects in your cluster. The controller (validates and) manages those experiments and trials, and optionally communicates with the RSO Server. The manager can be installed on your cluster using `redskyctl init` (see [Installation](install.md)).

`redskyctl` ("Red Sky Control") is a command-line tool for interacting with the manager. It can be used to install the manager, experiment with parameter suggestions interactively, and more. It is meant to be used in combination with `kubectl`.

If you are on a Red Sky Ops Enterprise plan, you can configure your cluster to connect to a RSO Server. The RSO Enterprise Server runs an experiment automatically, using machine learning to find the best combinations of parameters at the lowest cost.

## Concepts
- Parameters to tune
- Metrics to measure
- Patches to apply
- Link to full reference

### Experiments
An **Experiment** is the basic (unit of organization) (better term?) in Red Sky Ops. It consists of **parameters** to tune, one or more **metrics** to optimize, instructions for running the experiment, and optional **patches** to apply.

**Parameters** are the inputs to an experiment. They are declared in an experiment's `.spec.parameters`. A parameter has a minimum value, a maximum value, and a name. (Currently, only numeric parameters are supported.) (Is that true? Double-check, and see if we document it somewhere)

Taken together, an experiment's parameters define the **search space**: the total domain of all possible configurations.

**Metrics** are used to measure the result of a particular choice of parameters. A metric is a numeric value that an experiment attempts to minimize (like cost in dollars) or maximize (like throughput) by adjusting the values of parameters.

Experiments are run

### Trials
A **trial** is a single run of an experiment, with values assigned to every parameter. An experiment typically consists of many trials.

To get trials, (explain `redskyctl get experiment` etc, or maybe from `kubectl`)

## Timeline
Image from jeremy goes here.