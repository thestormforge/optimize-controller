
# API Docs



## Table of Contents
* [Assignment](#assignment)
* [ConfigMapHelmValuesFromSource](#configmaphelmvaluesfromsource)
* [HelmValue](#helmvalue)
* [HelmValueSource](#helmvaluesource)
* [HelmValuesFromSource](#helmvaluesfromsource)
* [ParameterSelector](#parameterselector)
* [PatchOperation](#patchoperation)
* [ReadinessCheck](#readinesscheck)
* [SetupTask](#setuptask)
* [Trial](#trial)
* [TrialCondition](#trialcondition)
* [TrialList](#triallist)
* [TrialReadinessGate](#trialreadinessgate)
* [TrialSpec](#trialspec)
* [TrialStatus](#trialstatus)
* [Value](#value)

## Assignment

Assignment represents an individual name/value pair. Assignment names must correspond to parameter names on the associated experiment.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `name` | Name of the parameter being assigned | _string_ | true |
| `value` | Value of the assignment | _intstr.IntOrString_ | true |

[Back to TOC](#table-of-contents)

## ConfigMapHelmValuesFromSource

ConfigMapHelmValuesFromSource is a reference to a ConfigMap that contains "*values.yaml" keys

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| _N/A_ |

[Back to TOC](#table-of-contents)

## HelmValue

HelmValue represents a value in a Helm template

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `name` | The name of Helm value as passed to one of the set options | _string_ | true |
| `forceString` | Force the value to be treated as a string | _bool_ | false |
| `value` | Set a Helm value using the evaluated template. Templates are evaluated using the same rules as patches | _intstr.IntOrString_ | false |
| `valueFrom` | Source for a Helm value | _*[HelmValueSource](#helmvaluesource)_ | false |

[Back to TOC](#table-of-contents)

## HelmValueSource

HelmValueSource represents a source for a Helm value

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `parameterRef` | Selects a trial parameter assignment as a Helm value | _*[ParameterSelector](#parameterselector)_ | false |

[Back to TOC](#table-of-contents)

## HelmValuesFromSource

HelmValuesFromSource represents a source of a values mapping

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `configMap` | The ConfigMap to select from | _*[ConfigMapHelmValuesFromSource](#configmaphelmvaluesfromsource)_ | false |

[Back to TOC](#table-of-contents)

## ParameterSelector

ParameterSelector selects a trial parameter assignment. Note that parameters values are used as is (i.e. in numeric form), for more control over the formatting of a parameter assignment use the template option on HelmValue.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `name` | The name of the trial parameter to use | _string_ | true |

[Back to TOC](#table-of-contents)

## PatchOperation

PatchOperation represents a patch used to prepare the cluster for a trial run, includes the evaluated parameter assignments as necessary

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `targetRef` | The reference to the object that the patched should be applied to | _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectreference-v1-core)_ | true |
| `patchType` | The patch content type, must be a type supported by the Kubernetes API server | _types.PatchType_ | true |
| `data` | The raw data representing the patch to be applied | _[]byte_ | true |
| `attemptsRemaining` | The number of remaining attempts to apply the patch, will be automatically set to zero if the patch is successfully applied | _int_ | false |

[Back to TOC](#table-of-contents)

## ReadinessCheck

ReadinessCheck represents a check to determine when the patched application is "ready" and it is safe to start the trial run job

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `targetRef` | TargetRef is the reference to the object to test the readiness of | _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectreference-v1-core)_ | true |
| `selector` | Selector may be used to trigger a search for multiple related objects to search; this may have RBAC implications, in particular "list" permissions are required | _*[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#labelselector-v1-meta)_ | false |
| `conditionTypes` | ConditionTypes are the status conditions that must be "True"; in addition to conditions that appear in the status of the target object, additional special conditions starting with "redskyops.dev/" can be tested | _[]string_ | false |
| `initialDelaySeconds` | InitialDelaySeconds is the approximate number of seconds after all of the patches have been applied to start evaluating this check | _int32_ | false |
| `periodSeconds` | PeriodSeconds is the approximate amount of time in between evaluation attempts of this check | _int32_ | false |
| `attemptsRemaining` | AttemptsRemaining is the number of failed attempts to allow before marking the entire trial as failed, will be automatically set to zero if the check has been successfully evaluated | _int32_ | false |
| `lastCheckTime` | LastCheckTime is the timestamp of the last evaluation attempt | _*[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#time-v1-meta)_ | false |

[Back to TOC](#table-of-contents)

## SetupTask

SetupTask represents the configuration necessary to apply application state to the cluster prior to each trial run and remove that state after the run concludes

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `name` | The name that uniquely identifies the setup task | _string_ | true |
| `image` | Override the default image used for performing setup tasks | _string_ | false |
| `command` | Override the default command for the container | _[]string_ | false |
| `args` | Override the default args for the container | _[]string_ | false |
| `skipCreate` | Flag to indicate the creation part of the task can be skipped | _bool_ | false |
| `skipDelete` | Flag to indicate the deletion part of the task can be skipped | _bool_ | false |
| `volumeMounts` | Volume mounts for the setup task | _[][VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#volumemount-v1-core)_ | false |
| `helmChart` | The Helm chart reference to release as part of this task | _string_ | false |
| `helmChartVersion` | The Helm chart version, empty means use the latest | _string_ | false |
| `helmValues` | The Helm values to set, ignored unless helmChart is also set | _[][HelmValue](#helmvalue)_ | false |
| `helmValuesFrom` | The Helm values, ignored unless helmChart is also set | _[][HelmValuesFromSource](#helmvaluesfromsource)_ | false |

[Back to TOC](#table-of-contents)

## Trial

Trial is the Schema for the trials API

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `metadata` | Standard object metadata | _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectmeta-v1-meta)_ | false |
| `spec` | Specification of the desired behavior for a trial | _[TrialSpec](#trialspec)_ | false |
| `status` | Current status of a trial | _[TrialStatus](#trialstatus)_ | false |

[Back to TOC](#table-of-contents)

## TrialCondition

TrialCondition represents an observed condition of a trial

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `type` | The condition type, e.g. "redskyops.dev/trial-complete" | _TrialConditionType_ | true |
| `status` | The status of the condition, one of "True", "False", or "Unknown | _corev1.ConditionStatus_ | true |
| `lastProbeTime` | The last known time the condition was checked | _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#time-v1-meta)_ | true |
| `lastTransitionTime` | The time at which the condition last changed status | _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#time-v1-meta)_ | true |
| `reason` | A reason code describing the why the condition occurred | _string_ | false |
| `message` | A human readable message describing the transition | _string_ | false |

[Back to TOC](#table-of-contents)

## TrialList

TrialList contains a list of Trial

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `metadata` | Standard list metadata | _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#listmeta-v1-meta)_ | false |
| `items` | The list of trials | _[][Trial](#trial)_ | true |

[Back to TOC](#table-of-contents)

## TrialReadinessGate

TrialReadinessGate represents a readiness check on one or more objects that must pass after patches have been applied, but before the trial run job can start

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `kind` | Kind of the readiness target | _string_ | false |
| `name` | Name of the readiness target, mutually exclusive with "Selector" | _string_ | false |
| `apiVersion` | APIVersion of the readiness target | _string_ | false |
| `selector` | Selector matches the resources whose condition must be checked, mutually exclusive with "Name" | _*[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#labelselector-v1-meta)_ | false |
| `conditionTypes` | ConditionTypes are the status conditions that must be "True" | _[]string_ | false |
| `initialDelaySeconds` | InitialDelaySeconds is the approximate number of seconds after all of the patches have been applied to start evaluating this check | _int32_ | false |
| `periodSeconds` | PeriodSeconds is the approximate amount of time in between evaluation attempts of this check; defaults to 10 seconds, minimum value is 1 second | _int32_ | false |
| `failureThreshold` | FailureThreshold is number of times that any of the specified ready conditions may be "False"; defaults to 3, minimum value is 1 | _int32_ | false |

[Back to TOC](#table-of-contents)

## TrialSpec

TrialSpec defines the desired state of Trial

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `experimentRef` | ExperimentRef is the reference to the experiment that contains the definitions to use for this trial, defaults to an experiment in the same namespace with the same name | _*[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectreference-v1-core)_ | false |
| `assignments` | Assignments are used to patch the cluster state prior to the trial run | _[][Assignment](#assignment)_ | false |
| `selector` | Selector matches the job representing the trial run | _*[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#labelselector-v1-meta)_ | false |
| `jobTemplate` | JobTemplate is the job template used to create trial run jobs | _*[JobTemplateSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#jobtemplatespec-v1beta1-batch)_ | false |
| `initialDelaySeconds` | InitialDelaySeconds is number of seconds to wait after a trial becomes ready before starting the trial run job | _int32_ | false |
| `startTimeOffset` | The offset used to adjust the start time to account for spin up of the trial run | _*metav1.Duration_ | false |
| `approximateRuntime` | The approximate amount of time the trial run should execute (not inclusive of the start time offset) | _*metav1.Duration_ | false |
| `ttlSecondsAfterFinished` | The minimum number of seconds before an attempt should be made to clean up the trial, if unset or negative no attempt is made to clean up the trial | _*int32_ | false |
| `ttlSecondsAfterFailure` | The minimum number of seconds before an attempt should be made to clean up a failed trial, defaults to TTLSecondsAfterFinished | _*int32_ | false |
| `readinessGates` | The readiness gates to check before running the trial job | _[][TrialReadinessGate](#trialreadinessgate)_ | false |
| `values` | Values are the collected metrics at the end of the trial run | _[][Value](#value)_ | false |
| `setupTasks` | Setup tasks that must run before the trial starts (and possibly after it ends) | _[][SetupTask](#setuptask)_ | false |
| `setupVolumes` | Volumes to make available to setup tasks, typically ConfigMap backed volumes | _[][Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#volume-v1-core)_ | false |
| `setupServiceAccountName` | Service account name for running setup tasks, needs enough permissions to add and remove software | _string_ | false |
| `setupDefaultClusterRole` | Cluster role name to be assigned to the setup service account when creating namespaces | _string_ | false |
| `setupDefaultRules` | Policy rules to be assigned to the setup service account when creating namespaces | _[]rbacv1.PolicyRule_ | false |

[Back to TOC](#table-of-contents)

## TrialStatus

TrialStatus defines the observed state of Trial

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `phase` | Phase is a brief human readable description of the trial status | _string_ | true |
| `assignments` | Assignments is a string representation of the trial assignments for reporting purposes | _string_ | true |
| `values` | Values is a string representation of the trial values for reporting purposes | _string_ | true |
| `startTime` | StartTime is the effective (possibly adjusted) time the trial run job started | _*[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#time-v1-meta)_ | false |
| `completionTime` | CompletionTime is the effective (possibly adjusted) time the trial run job completed | _*[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#time-v1-meta)_ | false |
| `conditions` | Condition is the current state of the trial | _[][TrialCondition](#trialcondition)_ | false |
| `patchOperations` | PatchOperations are the patches from the experiment evaluated in the context of this trial | _[][PatchOperation](#patchoperation)_ | false |
| `readinessChecks` | ReadinessChecks are the all of the objects whose conditions need to be inspected for this trial | _[][ReadinessCheck](#readinesscheck)_ | false |

[Back to TOC](#table-of-contents)

## Value

Value represents an observed metric value after a trial run has completed successfully. Value names must correspond to metric names on the associated experiment.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `name` | The metric name the value corresponds to | _string_ | true |
| `value` | The observed float64 value, formatted as a string | _string_ | true |
| `error` | The observed float64 error (standard deviation), formatted as a string | _string_ | false |
| `attemptsRemaining` | The number of remaining attempts to observer the value, will be automatically set to zero if the metric is successfully collected | _int_ | false |

[Back to TOC](#table-of-contents)
