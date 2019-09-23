
# API Docs



## Table of Contents
* [Experiment](#experiment)
* [ExperimentList](#experimentlist)
* [ExperimentSpec](#experimentspec)
* [ExperimentStatus](#experimentstatus)
* [Metric](#metric)
* [Parameter](#parameter)
* [PatchTemplate](#patchtemplate)
* [TrialTemplateSpec](#trialtemplatespec)

## Experiment

Experiment is the Schema for the experiments API

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `metadata` | Standard object metadata | _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectmeta-v1-meta)_ | false |
| `spec` | Specification of the desired behavior for an experiment | _[ExperimentSpec](#experimentspec)_ | false |
| `status` | Current status of an experiment | _[ExperimentStatus](#experimentstatus)_ | false |

[Back to TOC](#table-of-contents)

## ExperimentList

ExperimentList contains a list of Experiment

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `metadata` | Standard list metadata | _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#listmeta-v1-meta)_ | false |
| `items` | The list of experiments | _[][Experiment](#experiment)_ | true |

[Back to TOC](#table-of-contents)

## ExperimentSpec

ExperimentSpec defines the desired state of Experiment

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `replicas` | Replicas is the number of trials to execute concurrently, defaults to 1 | _*int32_ | false |
| `parallelism` | Parallelism is the total number of expected replicas across all clusters, defaults to the replica count | _*int32_ | false |
| `burnIn` | Burn-in is the number of trials using random suggestions at the start of an experiment | _*int32_ | false |
| `budget` | Budget is the maximum number of trials to run for an experiment across all clusters | _*int32_ | false |
| `parameters` | Parameters defines the search space for the experiment | _[][Parameter](#parameter)_ | false |
| `metrics` | Metrics defines the outcomes for the experiment | _[][Metric](#metric)_ | false |
| `patches` | Patches is a sequence of templates written against the experiment parameters that will be used to put the cluster into the desired state | _[][PatchTemplate](#patchtemplate)_ | false |
| `namespaceSelector` | NamespaceSelector is used to determine which namespaces on a cluster can be used to create trials. Only a single trial can be created in each namespace so if there are fewer matching namespaces then replicas, no trials will be created | _*[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#labelselector-v1-meta)_ | false |
| `selector` | Selector locates trial resources that are part of this experiment | _*[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#labelselector-v1-meta)_ | false |
| `template` | Template for creating a new trial. The resulting trial must be matched by Selector. The template can provide an initial namespace, however other namespaces (matched by NamespaceSelector) will be used if the effective replica count is more then one | _[TrialTemplateSpec](#trialtemplatespec)_ | true |

[Back to TOC](#table-of-contents)

## ExperimentStatus

ExperimentStatus defines the observed state of Experiment

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| _N/A_ |

[Back to TOC](#table-of-contents)

## Metric

Metric represents an observable outcome from a trial run

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `name` | The name of the metric | _string_ | true |
| `minimize` | Indicator that the goal of the experiment is to minimize the value of this metric | _bool_ | false |
| `type` | The metric collection type, one of: local\|prometheus\|jsonpath, default: local | _MetricType_ | false |
| `query` | Collection type specific query, e.g. Go template for "local", PromQL for "prometheus" or a JSON pointer expression (with curly braces) for "jsonpath" | _string_ | true |
| `errorQuery` | Collection type specific query for the error associated with collected metric value | _string_ | false |
| `scheme` | The scheme to use when collecting metrics | _string_ | false |
| `selector` | Selector matching services to collect this metric from, only the first matched service to provide a value is used | _*[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#labelselector-v1-meta)_ | false |
| `port` | The port number or name on the matched service to collect the metric value from | _intstr.IntOrString_ | false |
| `path` | URL path component used to collect the metric value from an endpoint (used as a prefix for the Prometheus API) | _string_ | false |

[Back to TOC](#table-of-contents)

## Parameter

Parameter represents the domain of a single component of the experiment search space

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `name` | The name of the parameter | _string_ | true |
| `min` | The inclusive minimum value of the parameter | _int64_ | false |
| `max` | The inclusive maximum value of the parameter | _int64_ | false |

[Back to TOC](#table-of-contents)

## PatchTemplate

PatchTemplate defines a target resource and a patch template to apply

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `type` | The patch type, one of: json\|merge\|strategic, default: strategic | _PatchType_ | false |
| `patch` | A Go Template that evaluates to valid patch. | _string_ | true |
| `targetRef` | Direct reference to the object the patch should be applied to. | _*[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectreference-v1-core)_ | false |

[Back to TOC](#table-of-contents)

## TrialTemplateSpec

TrialTemplateSpec is used as a template for creating new trials

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `metadata` | Standard object metadata | _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectmeta-v1-meta)_ | false |
| `spec` | Specification of the desired behavior for the trial | _[TrialSpec](#trialspec)_ | true |

[Back to TOC](#table-of-contents)
