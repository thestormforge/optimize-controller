/*
Copyright 2021 GramLabs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// DefaultCostWeights returns resource weightings for recognized special names.
func DefaultCostWeights(name string) corev1.ResourceList {
	switch strings.Map(toName, name) {
	case "cost":
		// TODO This should be smart enough to know if there is application wide cloud provider configuration
		return corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("17"),
			corev1.ResourceMemory: resource.MustParse("3"),
		}

	case "cost-gcp", "gcp-cost", "cost-gke", "gke-cost":
		return corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("17"),
			corev1.ResourceMemory: resource.MustParse("2"),
		}

	case "cost-aws", "aws-cost", "cost-eks", "eks-cost":
		return corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("18"),
			corev1.ResourceMemory: resource.MustParse("5"),
		}

	case "cpu-requests", "cpu":
		return corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1"),
		}

	case "memory-requests", "memory":
		return corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("1"),
		}

	default:
		return nil
	}
}
