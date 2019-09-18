# Introduction
## Introduction to Red Sky Ops
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

For an in-depth explanation of the tool, proceed to the Overview section.
