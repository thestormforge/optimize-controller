
# API Docs



## Table of Contents
* [AmazonWebServices](#amazonwebservices)
* [Application](#application)
* [ContainerResources](#containerresources)
* [GoogleCloudPlatform](#googlecloudplatform)
* [Ingress](#ingress)
* [LatencyObjective](#latencyobjective)
* [Objective](#objective)
* [Parameters](#parameters)
* [RequestsObjective](#requestsobjective)
* [Scenario](#scenario)
* [StormForger](#stormforger)
* [StormForgerAccessToken](#stormforgeraccesstoken)
* [StormForgerScenario](#stormforgerscenario)

## AmazonWebServices

AmazonWebServices is used to configure details specific to applications hosted in AWS.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| _N/A_ |

[Back to TOC](#table-of-contents)

## Application

Application represents a description of an application to run experiments on.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `metadata` |  | _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectmeta-v1-meta)_ | false |
| `resources` | Resources are references to application resources to consider in the generation of the experiment. These strings are the same format as used by Kustomize. | _[]string_ | false |
| `parameters` | Parameters specifies additional details about the experiment parameters. | _*[Parameters](#parameters)_ | false |
| `ingress` | Ingress specifies how to find the entry point to the application. | _*[Ingress](#ingress)_ | false |
| `scenarios` | The list of scenarios to optimize the application for. | _[][Scenario](#scenario)_ | false |
| `objectives` | The list of objectives to optimizat the application for. | _[][Objective](#objective)_ | false |
| `googleCloudPlatform` | GoogleCloudPlatform allows you to configure hosting details specific to GCP. | _*[GoogleCloudPlatform](#googlecloudplatform)_ | false |
| `amazonWebServices` | AmazonWebServices allows you to configure hosting details specific to AWS. | _*[AmazonWebServices](#amazonwebservices)_ | false |
| `stormForger` | StormForger allows you to configure StormForger to apply load on your application. | _*[StormForger](#stormforger)_ | false |

[Back to TOC](#table-of-contents)

## ContainerResources

ContainerResources specifies which resources in the application should have their container resources (CPU and memory) optimized.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `labels` | Labels of Kubernetes objects to consider when generating container resources patches. | _map[string]string_ | false |

[Back to TOC](#table-of-contents)

## GoogleCloudPlatform

GoogleCloudPlatform is used to configure details specific to applications hosted in GCP.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| _N/A_ |

[Back to TOC](#table-of-contents)

## Ingress

Ingress describes the point of ingress to the application.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `url` | The URL used to access the application from outside the cluster. | _string_ | false |

[Back to TOC](#table-of-contents)

## LatencyObjective

LatencyObject is used to optimize the responsiveness of an application in a specific scenario.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `LatencyType` | The latency to optimize. | _LatencyType_ | false |

[Back to TOC](#table-of-contents)

## Objective

Objective describes the goal of the optimization in terms of specific metrics.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `name` | The name of the objective. If no objective specific configuration is supplied, the name is used to derive a configuration. | _string_ | true |
| `max` | The upper bound for the objective. | _*resource.Quantity_ | false |
| `min` | The lower bound for the objective. | _*resource.Quantity_ | false |
| `optimize` | Flag indicating that this objective should optimized instead of monitored (default: true). | _*bool_ | false |
| `requests` | Requests is used to optimize the resources consumed by an application. | _*[RequestsObjective](#requestsobjective)_ | false |
| `latency` | Latency is used to optimize the responsiveness of an application. | _*[LatencyObjective](#latencyobjective)_ | false |

[Back to TOC](#table-of-contents)

## Parameters

Parameters describes the strategy for tuning the application.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `containerResources` | Information related to the discovery of container resources parameters like CPU and memory. | _*[ContainerResources](#containerresources)_ | false |

[Back to TOC](#table-of-contents)

## RequestsObjective

RequestsObjective is used to optimize the resource requests of an application in a specific scenario.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `labels` | Labels of the pods which should be considered when collecting cost information. | _map[string]string_ | false |
| `weights` | Weights are used to determine which container resources should be optimized. | _corev1.ResourceList_ | false |

[Back to TOC](#table-of-contents)

## Scenario

Scenario describes a specific pattern of load to optimize the application for.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `name` | The name of scenario. | _string_ | true |
| `stormforger` | StormForger configuration for the scenario. | _*[StormForgerScenario](#stormforgerscenario)_ | false |

[Back to TOC](#table-of-contents)

## StormForger

StormForger describes global configuration related to StormForger.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `org` | The name of the StormForger organization. | _string_ | false |
| `accessToken` | Configuration for the StormForger service account. | _*[StormForgerAccessToken](#stormforgeraccesstoken)_ | false |

[Back to TOC](#table-of-contents)

## StormForgerAccessToken

StormForgerAccessToken is used to configure a service account access token for the StormForger API.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `file` | The path to the file that contains the service account access token. | _string_ | false |
| `literal` | A literal token value, this should only be used for testing as it is not secure. | _string_ | false |
| `secretKeyRef` | Reference to an existing secret key that contains the access token. | _*corev1.SecretKeySelector_ | false |

[Back to TOC](#table-of-contents)

## StormForgerScenario

StormForgerScenario is used to generate load using StormForger.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `testCase` | Override the generated test case name. | _string_ | false |
| `testCaseFile` | Path to the test case file. | _string_ | false |

[Back to TOC](#table-of-contents)
