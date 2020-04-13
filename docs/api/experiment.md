
# API Docs



## Table of Contents
* [Experiment](#experiment)
* [ExperimentList](#experimentlist)
* [ExperimentSpec](#experimentspec)
* [ExperimentStatus](#experimentstatus)
* [Metric](#metric)
* [NamespaceTemplateSpec](#namespacetemplatespec)
* [Optimization](#optimization)
* [Parameter](#parameter)
* [PatchReadinessGate](#patchreadinessgate)
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
| `optimization` | Optimization defines additional configuration for the optimization | _[][Optimization](#optimization)_ | false |
| `parameters` | Parameters defines the search space for the experiment | _[][Parameter](#parameter)_ | false |
| `metrics` | Metrics defines the outcomes for the experiment | _[][Metric](#metric)_ | false |
| `patches` | Patches is a sequence of templates written against the experiment parameters that will be used to put the cluster into the desired state | _[][PatchTemplate](#patchtemplate)_ | false |
| `namespaceSelector` | NamespaceSelector is used to locate existing namespaces for trials | _*[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#labelselector-v1-meta)_ | false |
| `namespaceTemplate` | NamespaceTemplate can be specified to create new namespaces for trials; if specified created namespaces must be matched by the namespace selector | _*[NamespaceTemplateSpec](#namespacetemplatespec)_ | false |
| `selector` | Selector locates trial resources that are part of this experiment | _*[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#labelselector-v1-meta)_ | false |
| `template` | Template for creating a new trial. The resulting trial must be matched by Selector. The template can provide an initial namespace, however other namespaces (matched by NamespaceSelector) will be used if the effective replica count is more then one | _[TrialTemplateSpec](#trialtemplatespec)_ | true |

[Back to TOC](#table-of-contents)

## ExperimentStatus

ExperimentStatus defines the observed state of Experiment

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `phase` | Phase is a brief human readable description of the experiment status | _string_ | true |
| `activeTrials` | ActiveTrials is the observed number of running trials | _int32_ | true |

[Back to TOC](#table-of-contents)

## Metric

Metric represents an observable outcome from a trial run

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `name` | The name of the metric | _string_ | true |
| `minimize` | Indicator that the goal of the experiment is to minimize the value of this metric | _bool_ | false |
| `type` | The metric collection type, one of: local\|prometheus\|datadog\|jsonpath, default: local | _MetricType_ | false |
| `query` | Collection type specific query, e.g. Go template for "local", PromQL for "prometheus" or a JSON pointer expression (with curly braces) for "jsonpath" | _string_ | true |
| `errorQuery` | Collection type specific query for the error associated with collected metric value | _string_ | false |
| `scheme` | The scheme to use when collecting metrics | _string_ | false |
| `selector` | Selector matching services to collect this metric from, only the first matched service to provide a value is used | _*[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#labelselector-v1-meta)_ | false |
| `port` | The port number or name on the matched service to collect the metric value from | _intstr.IntOrString_ | false |
| `path` | URL path component used to collect the metric value from an endpoint (used as a prefix for the Prometheus API) | _string_ | false |

[Back to TOC](#table-of-contents)

## NamespaceTemplateSpec

NamespaceTemplateSpec is used as a template for creating new namespaces

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `metadata` | Standard object metadata | _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectmeta-v1-meta)_ | false |
| `spec` | Specification of the namespace | _corev1.NamespaceSpec_ | false |

[Back to TOC](#table-of-contents)

## Optimization

Optimization is a configuration setting for the optimizer

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `name` | Name is the name of the optimization configuration to set | _string_ | true |
| `value` | Value is string representation of the optimization configuration | _string_ | true |

[Back to TOC](#table-of-contents)

## Parameter

Parameter represents the domain of a single component of the experiment search space

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `name` | The name of the parameter | _string_ | true |
| `min` | The inclusive minimum value of the parameter | _int64_ | false |
| `max` | The inclusive maximum value of the parameter | _int64_ | false |
| `upperParameter` | The name of a parameter that defines the upper bound for this parameter | _string_ | false |
| `lowerParameter` | The name of a parameter that defines the lower bound for this parameter | _string_ | false |

[Back to TOC](#table-of-contents)

## PatchReadinessGate

PatchReadinessGate contains a reference to a condition

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `conditionType` | ConditionType refers to a condition in the patched target's condition list | _string_ | true |

[Back to TOC](#table-of-contents)

## PatchTemplate

PatchTemplate defines a target resource and a patch template to apply

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `type` | The patch type, one of: json\|merge\|strategic, default: strategic | _PatchType_ | false |
| `patch` | A Go Template that evaluates to valid patch. | _string_ | true |
| `targetRef` | Direct reference to the object the patch should be applied to. | _*[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectreference-v1-core)_ | false |
| `readinessGates` | ReadinessGates will be evaluated for patch target readiness. A patch target is ready if all conditions specified in the readiness gates have a status equal to "True". If no readiness gates are specified, some target types may have default gates assigned to them. Some condition checks may result in errors, e.g. a condition type of "Ready" is not allowed for a ConfigMap. Condition types starting with "redskyops.dev/" may not appear in the patched target's condition list, but are still evaluated against the resource's state. | _[][PatchReadinessGate](#patchreadinessgate)_ | false |

[Back to TOC](#table-of-contents)

## TrialTemplateSpec

TrialTemplateSpec is used as a template for creating new trials

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `metadata` | Standard object metadata | _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectmeta-v1-meta)_ | false |
| `spec` | Specification of the desired behavior for the trial | _[TrialSpec](#trialspec)_ | false |

[Back to TOC](#table-of-contents)
