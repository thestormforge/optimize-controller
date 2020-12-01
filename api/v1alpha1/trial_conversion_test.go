/*
Copyright 2020 GramLabs, Inc.

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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thestormforge/optimize-controller/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestTrial_ConvertRoundTrip(t *testing.T) {
	now := metav1.Now()
	one := int32(1)
	oneSecond := metav1.Duration{Duration: 1 * time.Second}
	cases := []struct {
		desc string
		t    *Trial
	}{
		{
			desc: "empty",
			t:    &Trial{},
		},
		{
			desc: "configuration",
			t: &Trial{
				Spec: TrialSpec{
					ExperimentRef: &corev1.ObjectReference{
						Kind:      "Tester",
						Namespace: "default",
						Name:      "test",
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					},
					ApproximateRuntime:      &oneSecond,
					TTLSecondsAfterFinished: &one,
					ReadinessGates: []TrialReadinessGate{
						{
							Kind: "Tester",
							Name: "test",
							Selector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
							},
							ConditionTypes:      []string{"Ready"},
							InitialDelaySeconds: 1,
						},
					},
				},
			},
		},
		{
			desc: "job",
			t: &Trial{
				Spec: TrialSpec{
					Template: &batchv1beta1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							BackoffLimit: &one,
							Template:     corev1.PodTemplateSpec{},
						},
					},
				},
			},
		},
		{
			desc: "assignments",
			t: &Trial{
				Spec: TrialSpec{
					Assignments: []Assignment{
						{
							Name:  "tp",
							Value: intstr.FromInt(1),
						},
						{
							Name:  "tp2",
							Value: intstr.FromInt(2),
						},
					},
				},
				Status: TrialStatus{
					Assignments: "tp=1, tp2=2",
				},
			},
		},
		{
			desc: "patches",
			t: &Trial{
				Spec: TrialSpec{
					PatchOperations: []PatchOperation{
						{
							TargetRef: corev1.ObjectReference{
								Kind:      "Tester",
								Namespace: "default",
								Name:      "test",
							},
							PatchType:         "test",
							Data:              []byte("test: true"),
							AttemptsRemaining: 1,
						},
					},
				},
			},
		},
		{
			desc: "checks",
			t: &Trial{
				Spec: TrialSpec{
					ReadinessChecks: []ReadinessCheck{
						{
							TargetRef: corev1.ObjectReference{
								Kind:      "Tester",
								Namespace: "default",
								Name:      "test",
							},
							Selector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
							},
							ConditionTypes:      []string{"Ready"},
							InitialDelaySeconds: 1,
							PeriodSeconds:       1,
							AttemptsRemaining:   1,
							LastCheckTime:       &now,
						},
					},
				},
			},
		},
		{
			desc: "values",
			t: &Trial{
				Spec: TrialSpec{
					Values: []Value{
						{
							Name:  "tm",
							Value: "1.0",
						},
						{
							Name:              "tm2",
							Value:             "1.0",
							AttemptsRemaining: 1,
						},
					},
				},
				Status: TrialStatus{
					Values: "tm=1.0",
				},
			},
		},
		{
			desc: "conditions",
			t: &Trial{
				Status: TrialStatus{
					StartTime:      &now,
					CompletionTime: &now,
					Conditions: []TrialCondition{
						{
							Type:               "Testing",
							Status:             "Unknown",
							LastProbeTime:      now,
							LastTransitionTime: now,
							Reason:             "Testing",
						},
					},
				},
			},
		},
		{
			desc: "helmValues",
			t: &Trial{
				Spec: TrialSpec{
					SetupTasks: []SetupTask{
						{
							Name:      "helmvalue",
							HelmChart: "foobar",
							HelmValues: []HelmValue{
								{
									Name:  "val",
									Value: intstr.FromInt(1),
								},
								{
									Name: "valFrom",
									ValueFrom: &HelmValueSource{
										ParameterRef: &ParameterSelector{
											Name: "tp",
										},
									},
								},
							},
							HelmValuesFrom: []HelmValuesFromSource{
								{
									ConfigMap: &ConfigMapHelmValuesFromSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "cm",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "setupVolumes",
			t: &Trial{
				Spec: TrialSpec{
					SetupTasks: []SetupTask{
						{
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "setupvolume",
									MountPath: "/foobar",
								},
							},
						},
					},
					SetupVolumes: []corev1.Volume{
						{
							Name: "setupvolume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: resource.NewMilliQuantity(1024, resource.BinarySI),
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "setupRules",
			t: &Trial{
				Spec: TrialSpec{
					SetupDefaultRules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Verbs:     []string{"get"},
							Resources: []string{"configmaps"},
						},
					},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			var err error
			src := &Trial{}
			hub := &v1beta1.Trial{}

			// Convert to the hub version
			err = c.t.ConvertTo(hub)
			if !assert.NoError(t, err) {
				return
			}

			// Convert back to the source version
			err = src.ConvertFrom(hub)
			if !assert.NoError(t, err) {
				return
			}

			// They should be the same
			assert.Equal(t, c.t, src)
		})
	}
}
