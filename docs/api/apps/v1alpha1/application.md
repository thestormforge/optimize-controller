
# API Docs



## Table of Contents
* [AmazonWebServices](#amazonwebservices)
* [Application](#application)
* [CloudProvider](#cloudprovider)
* [ContainerResources](#containerresources)
* [CostObjective](#costobjective)
* [GenericCloudProvider](#genericcloudprovider)
* [GoogleCloudPlatform](#googlecloudplatform)
* [Ingress](#ingress)
* [Objective](#objective)
* [Parameters](#parameters)
* [Scenario](#scenario)

## AmazonWebServices

AmazonWebServices is used to configure details specific to applications hosted in AWS.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `cost` | Per-resource cost weightings. | _corev1.ResourceList_ | false |

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
| `cloudProvider` | CloudProvider is used to provide details about the hosting environment the application is run in. | _*[CloudProvider](#cloudprovider)_ | false |

[Back to TOC](#table-of-contents)

## CloudProvider

CloudProvider describes the how the application is being hosted.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `gcp` | Configuration specific to Google Cloud Platform. | _*[GoogleCloudPlatform](#googlecloudplatform)_ | false |
| `aws` | Configuration specific to Amazon Web Services. | _*[AmazonWebServices](#amazonwebservices)_ | false |

[Back to TOC](#table-of-contents)

## ContainerResources

ContainerResources specifies which resources in the application should have their container resources (CPU and memory) optimized.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `labels` | Labels of Kubernetes objects to consider when generating container resources patches. | _map[string]string_ | false |

[Back to TOC](#table-of-contents)

## CostObjective

CostObjective is used to estimate the cost of running an application in a specific scenario.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `labels` | Labels of the pods which should be considered when collecting cost information. | _map[string]string_ | false |

[Back to TOC](#table-of-contents)

## GenericCloudProvider

GenericCloudProvider is used to configure details for applications hosted on other platforms.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `cost` | Per-resource cost weightings. | _corev1.ResourceList_ | false |

[Back to TOC](#table-of-contents)

## GoogleCloudPlatform

GoogleCloudPlatform is used to configure details specific to applications hosted in GCP.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `cost` | Per-resource cost weightings. | _corev1.ResourceList_ | false |

[Back to TOC](#table-of-contents)

## Ingress

Ingress describes the point of ingress to the application.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `serviceName` | The name of the service to use for ingress to the application. | _string_ | false |

[Back to TOC](#table-of-contents)

## Objective

Objective describes the goal of the optimization in terms of specific metrics.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `name` | The name of the objective. | _string_ | true |
| `cost` | Cost is used to identify which parts of the application impact the cost of running the application. | _*[CostObjective](#costobjective)_ | false |

[Back to TOC](#table-of-contents)

## Parameters

Parameters describes the strategy for tuning the application.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `containerResources` | Information related to the discovery of container resources parameters like CPU and memory. | _*[ContainerResources](#containerresources)_ | false |

[Back to TOC](#table-of-contents)

## Scenario

Scenario describes a specific pattern of load to optimize the application for.

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| `name` | The name of scenario. | _string_ | true |

[Back to TOC](#table-of-contents)
