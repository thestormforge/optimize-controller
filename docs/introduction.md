
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

### Introduction
`k8s-experiment` is a tool for experimenting with object configurations in Kubernetes. With `k8s-experiment` installed on your cluster, you can run experiments that manipulate resources and measure the outcome via one or more metrics.

When used as part of Red Sky Ops, `k8s-experiment` tunes your Kubernetes applications automatically using machine learning. Without the enterprise server, `k8s-experiment` can be used to manually experiment on your cluster, or can be integrated into existing automation workflows.

Although `k8s-experiment` was developed as part of Red Sky Ops to tune performance, it can run non-performance-related experiments on your cluster as well. This documentation describes the concepts and capabilities of `k8s-experiment` generically, and typically includes performance-related examples for relevant concepts.

(Call to action here? "Using RSO in a novel way? Tell us about it!" link to github, twitter?)

### Background: Managing Compute Resources for Containers
(Title from K8s docs: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#extended-resources)

Kubernetes has several built-in mechanisms for managing container compute resources. To begin, Pods may specify CPU and memory requests and limits for each of their containers.

Once containers have these limits set, Kubernetes can leverage them in several ways:

- The Kubernetes scheduler uses the resource requests to schedule pods onto nodes ("bin packing")
- When pods exceed their limits for CPU (or memory?), they may be evicted from nodes.
- When used in combination with Quality of Service (QoS) classes or pod priority, Kubernetes can go a step further, choosing which pods to schedule and evict pods based on the quality of service they require
- Horizontal Pod Autoscaling. To accommodate demand on pods, kubernetes can "horizontally" scale them out by adding more replicas. The HPA controller (check this) examines the CPU resource request and calculates utilization as a fraction of the requested amount to determine when to scale.

Does not address getting the CPU, mem requests / limits correct (Vertical Pod Scaling), and leaves a lot on the table with deeper application parameters (Logstash, elastic/JVM examples...)

Moreover, many applications support additional application-specific parameters that are not exposed through a Kubernetes. Examples in commonly used applications include Logstash's batch size and batch delay; $another. Your own application may have been built with similar parameters.

Tuning these application-level parameters enables deeper optimization of applications.

Aside: Why not just HPA?
- Scales up, but not down. Meant to scale up to demand, and back down, but not to prevent underutilization per pod.

`k8s-experiment` goes beyond HPA, providing deeper and more holistic tuning capabilities across an application.

It provides all of the tools needed to find the ideal configuration for your application. It support parameterizing application yaml; running trials with specific values filled into those yaml documents; taking measurements to determine the efficacy of trials; and grouping those trials into overarching Experiments.

For an in-depth explanation of the tool, proceed to the [Concepts]() section.


# Concepts
- Architecture
	- manager
	- redskyctl
	- optional server
- Experiments
- Trials
- Timeline of execution

## Draft
### Architecture
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