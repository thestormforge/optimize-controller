# Using Metrics

Trial metrics are collected immediately following completion of the trial run job, typically from a dedicated metric store like Prometheus. A single 64-bit floating point number is collected for each metric defined on the experiment.

As a general strategy, metrics should be selected with opposing goals: for example, choosing to minimize CPU usage and memory usage will optimize for an application that does not start and therefore does not use any CPU or memory at all. An example of opposing goals would be to minimize overall resource usage (a combined metric for both CPU and memory) while maximizing throughput of some part of the application.

## Defining Metrics

Metrics are defined using the `metrics` field on the experiment. Each metric in the list must have a unique name. Metrics also include a `type` that determines how the metric will be collected after the trial run job.

Other fields on the metric definition are used to control behavior of collection and may be interpreted differently for each type; for example, when using the `prometheus` metric type, the `query` field is treated as a PromQL query.

### Queries

Regardless of the query type, the `query` field is always preprocessed as a Go template, allowing the exact contents of the query to be evaluated after the trial is complete. For example, a PromQL query can be written to include a placeholder for the "range" (duration) of the trial run.

The following variables are defined for use in query processing:

| Variable Name     |  Type              | Description                                   |
|-------------------|--------------------|-----------------------------------------------|
| `Trial.Name`      | `string`           | The name of the trial                         |
| `Trial.Namespace` | `string`           | The namespace the trial ran in                |
| `Values`          | `map[string]int64` | The parameter assignments                     |
| `StartTime`       | `time`             | The adjusted start time of the trial run job  |
| `CompletionTime`  | `time`             | The completion time of the trial run job      |
| `Range`           | `string`           | The duration of the trial run job, e.g. "5s"  |
| `Pods`            | `PodList`          | The list of pods in the trial namespace       |

### Local Collection Type

The `"local"` (default) type simply parses the `query` field as a floating point number. Combined with the standard preprocessing of the query as a Go template, it is possible to capture values like the (potentially adjusted) trial run job execution time:

```yaml
  metrics:
    - name: time
      minimize: true
      query: "{{duration .StartTime .CompletionTime}}"
```

In this example, the `duration` template function is used to subtract the start time from the completion time of the trial.

### Pods Collection Type

The `"pods"` collection type is similar to the local type in that the evaluated query is expected to be a floating point number. However, the template data is given a list of pod definitions matching the metric selector.

### Prometheus Collection Type

The `"prometheus"` collection type treats the `query` field as a [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) query to execute against a Prometheus instance identified using a service selector. The `Range` template variable can be used when writing the PromQL to produce queries over the time interval during which the trial job was running; e.g. `[{{ .Range }}]`.

All Prometheus metrics must evaluate to scalar, that is a single floating point number. Often times it may be necessary to write a query that produces a single-element instant vector and extract that value using the [`scalar`](https://prometheus.io/docs/prometheus/latest/querying/functions/#scalar) function. Note that `scalar` function produces a `NaN` result when the size of the instant vector is not 1 and this will cause the trial to fail during metric collection.

When using the Prometheus collection type, the `selector` field is used to determine the instance of Prometheus to use. A cluster wide search (all namespaces) is performed for services matching the selector. In the case of multiple matched services, each service returned by the API server is tried until the first successful attempt to capture the metric value.

Prometheus connection information can be further refined using the `scheme` (must be `"https"` or `"http"`, the later of which is used by default), the `port` (a port number or name specified on the service, if the service only specifies one port this can be omitted) and the `path` (the context root of the Prometheus API).

### Datadog Collection Type

The `"datadog"` collection can be used to execute metric queries against the Datadog API.

In order to authenticate to the Datadog API, the `DATADOG_API_KEY` and `DATADOG_APP_KEY` environment variables must be set on the manager deployment. You can populate these environment variables during initialization by adding them to your configuration:

```sh
redskyctl config set controller.default.env.DATADOG_API_KEY xxx-yyy-zzz
redskyctl config set controller.default.env.DATADOG_APP_KEY xxx-yyy-zzz
```

Alternately you can manually edit your `~/.config/redsky/config` configuration file to include the following snippet:

```yaml
controllers:
  - name: default
    controller:
      env:
        - name: DATADOG_API_KEY
          value: xxx-yyy-zzz
        - name: DATADOG_APP_KEY
          value: xxx-yyy-zzz
```

Datadog metrics are subject to further aggregation (in addition to the aggregation method of the query); this is similar to the [Query Value](https://docs.datadoghq.com/graphing/widgets/query_value/) widget. By default, the `avg` aggregator is used, however this can be overridden by setting the `scheme` field of the metric to any of the supported aggregator values (avg, last, max, min, sum).

### JSONPath Collection Type

The `"jsonpath"` collection type fetches a JSON payload from an arbitrary HTTP endpoint and evaluates a [Kubernetes JSONPath](https://kubernetes.io/docs/reference/kubectl/jsonpath/) expression from the `query` field against it.

The result of the JSONPath expression must be a numeric value (or a string that can be parsed as floating point number), this typically means that the value of the metric `query` field _should_ start and end with curly braces, e.g. `"{.example.foobar}"` (since the `$` operator is optional).

When using the JSONPath collection type, the `selector` field is used to determine the HTTP endpoint to query. Conversely, the `scheme`, `port` and `path` fields can be used to refine the resulting URL. Note that query parameters are allowed in the `path` field if necessary: in general a request for the URL constructed from the template `{scheme}://{selectedServiceClusterIP}:{port}/{path}` is used with an `Accept: application/json` header to retrieve the JSON entity body.
