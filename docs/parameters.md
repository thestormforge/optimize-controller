# Using Parameters

Experiment parameters define the search space for assigned values that vary for each trial run. Each parameter represents a named integer assignment with an inclusive minimum and maximum bound.

## Parameter Domain

When selecting the bounds for a parameter it is important to remember that all values are configured as integers. When tuning compute resources, it may be useful to limit the domain of the parameter and adjust it according when applying the parameter value later.

If you are tuning a [CPU request](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-cpu) and would like to experiment with values from 100 millicpu to 4 CPU in increments of 100 millicpu you would typically use values like 0.1, 0.2, 0.3, ..., 4.0. When configuring your experiment, you should use a parameter with a minimum of 1 and maximum of 40 and later multiply that parameter value by 100 to achieve the desired final number. For example, given the parameter definition of:

```yaml
...
  parameters:
  - name: cpu
    min: 1
    max: 40
...
```

You would consume this parameter in a `spec.containers[].resources` patch of:

```yaml
                requests:
                  cpu: "{{ mul .Values.cpu 100 }}m"
```

To get values 100m, 200m, 300m, ..., 4000m in their preferred form.
