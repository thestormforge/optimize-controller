# Parameters

A **parameter** is an input to an experiment which can be adjusted from trial to trial. Every parameter has a name and a domain of acceptable values. The value given to a parameter in a specific trial is sometimes called the **parameter assignment**.

An experiment must define one or more parameters. Together, the parameters in an experiment define the **search space**.

## Parameter Types

Parameters can be one of two types: **Integer** or **String**.

### Integer Parameters

Integer parameters specify a minimum bound, a maximum bound, or both. (Both bounds are *inclusive*, and when either bound is unspecified, it defaults to 0.) A parameter for tuning CPU on a container might be defined with both bounds:

```yaml
  parameters:
  - name: cpu
    min: 100
    max: 4000
```

While a parameter for tuning the concurrent garbage collection threads on a Java application might be defined with only a `max` bound:

```yaml
  parameters:
  - name: con_gc_threads
    max: 8
```

*Note:* Some fields in Kubernetes objects, like [CPU Request](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#meaning-of-cpu), allow their value to be specified using decimal numbers like `0.1` or as an integer with an attached unit, like `100m` ("one hundred millicpus"). Because Red Sky Ops only tunes integer numbers, you must use the latter format. See [Using Parameters](#using-parameters).

### String Parameters

String parameters specify a finite list of acceptable values, where each value is a string. For example, a parameter for tuning the garbage collection algorithm on a Java application might be defined like:

```yaml
  parameters:
  - name: gc_collector
    values:
    - "G1"
    - "ConcMarkSweep"
    - "Serial"
    - "Parallel"
```

## Using Parameters

The `parameters` section of an experiment file defines the names and domains of the experiment's parameters. The `patches` section defines how those parameters are used in your application.

To continue with an earlier example, an integer parameter for tuning CPU on a container might be defined as follows:

```yaml
  parameters:
  - name: cpu
    min: 100
    max: 4000
```

This parameter would be consumed in a patch of a Deployment's `spec.template.spec.containers[].resources` field:

```yaml
  patches:
  - patch: |
      spec:
        template:
          spec:
            containers:
            - name: container-name
              resources:
                requests:
                  cpu: "{{ .Values.cpu }}m"
```

Note the inclusion of an `m` to specify millicpus.

### Parameter Manipulation

Sometimes it is necessary to manipulate a value to consume it in a patch. To support this, patches are evaluated as [Go templates](https://golang.org/pkg/text/template/), with [Sprig](http://masterminds.github.io/sprig/) template functions included to support a variety of operations. Additional template functions are also available:

- **percent**
  Return the integer percentage.

  `percent 9 50` will return `"4"`.

## Parameter Constraints

**Constraints** restrict the assignments that parameters may take relative to one another. Constraints are optional and used in addition to the bounds on individual parameters like `min` or `max`.

Red Sky Ops supports two types of constraints: **Order** and **Sum**.

### Order Constraint

The order constraint requires that one integer parameter be strictly larger than another. For example, you can use an order constraint to ensure that a container's maximum replicas value is always greater than it's minimum replicas:

```yaml
  constraints:
    - order:
        lowerParameter: min_replicas
        upperParameter: max_replicas
```

### Sum Constraint

The sum constraint requires that the sum of two or more parameters not exceed an upper or lower bound. For example, in an experiment tuning CPU on multiple deployments, a sum constraint could enforce an overall cap of `4000` millicpus between both deployments:

```yaml
  constraints:
    - sum:
        bound: 4000
        isUpperBound: true
        parameters:
          - name: first_cpu
          - name: second_cpu
```

Each parameter in the sum constraint may have an associated **weight**. When a weight is specified for a parameter, the parameter's value is multiplied by that weight before being summed.
