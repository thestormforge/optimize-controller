# Using Parameters

Experiment parameters define the search space for assigned values that vary for each trial run. Each parameter represents a named integer assignment with an inclusive minimum and maximum bound.

## Parameter Domain

When selecting the bounds for a parameter it is important to remember that all values are configured as integers. When tuning compute resources, such as a [CPU request](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-cpu), you may typically use values like 0.1, 0.2, 0.3, ..., 4.0. However, for optimization you will need to specify your bounds using millicpus and add the explicit unit later, for example:

```yaml
  parameters:
  - name: cpu
    min: 100
    max: 4000
```

You would consume this parameter in a `spec.containers[].resources` patch of:

```yaml
  requests:
    cpu: "{{ .Values.cpu }}m"
```

## Parameter Manipulation

All parameters are suggested as integer values, sometimes it is necessary to manipulate a value to consume it in a patch. Patches are evaluated as [Go templates](https://golang.org/pkg/text/template/) with the added [Sprig](http://masterminds.github.io/sprig/) template functions. Additional template functions are also available:

- **percent**
  Return the integer percentage.

  `percent 9 50` will return `"4"`
