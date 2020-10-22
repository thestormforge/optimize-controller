# Using Parameters

A **parameter** is a {value, field, placeholder, input to an experiment} which can be adjusted during an experiment. Every parameter has a name and a **domain** of acceptable values it can be **assigned** during a trial. 

An experiment must define one or more parameters. Together, the parameters in an experiment define the **search space**.

## Parameter Types

Red Sky Ops supports two types of parameters: **Integer** and **Categorical**.

### Integer

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

*Note:* Some fields in Kubernetes objects, like [CPU Request](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#meaning-of-cpu), allow their value to be specified using decimal numbers like `0.1` or as an integer with an attached unit, like `100m` ("one hundred millicpus"). Because Red Sky Ops only tunes integer numbers, you must use the latter format, as explained further in **Using Parameters**. 

### Categorical

Categorical parameters specify a finite list of acceptable values, where each value is a string. For example, a parameter for tuning the garbage collection algorithm on a Java application might be defined like:

```yaml
  parameters:
  - name: gc_collector
    values:
    - G1
    - ConcMarkSweep
    - Serial
    - Parallel
```

## Using Parameters

The `parameters` section of an experiment file defines the names and domains of the experiment's parameters. The `patches` section defines how those parameters are used in your application.

To reuse an earlier example, an integer parameter for tuning CPU on a container might be defined as follows:

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

Sometimes it is necessary to manipulate a parameter value to consume it in a patch. To support this, patches are evaluated as [Go templates](https://golang.org/pkg/text/template/) with the added [Sprig](http://masterminds.github.io/sprig/) template functions. Additional template functions are also available:

- **percent**
  Return the integer percentage.

  `percent 9 50` will return `"4"`
