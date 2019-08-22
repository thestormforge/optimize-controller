/*
Copyright 2019 GramLabs, Inc.

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
package controllers

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/redskyops/k8s-experiment/pkg/api/redsky/v1alpha1"
	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Experiment Controller", func() {

	const timeout = time.Second * 30
	const interval = time.Second * 1

	Context("Empty experiment", func() {
		It("Should create successfully", func() {
			var err error

			key := types.NamespacedName{
				Name:      "foo",
				Namespace: "default",
			}

			spec := redskyv1alpha1.ExperimentSpec{}

			instance := &redskyv1alpha1.Experiment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: spec,
			}

			Expect(k8sClient.Create(context.Background(), instance)).Should(Succeed())

			By("Expecting experiment URL to be set")
			Eventually(func() string {
				e := &redskyv1alpha1.Experiment{}
				_ = k8sClient.Get(context.Background(), key, e)
				return e.GetAnnotations()[redskyv1alpha1.AnnotationExperimentURL]
			}, timeout, interval).ShouldNot(BeEmpty())

			_, err = redSkyAPI.GetExperimentByName(context.Background(), v1alpha1.NewExperimentName(key.Name))
			Expect(err).Should(Succeed())

			By("Expecting to delete successfully")
			Eventually(func() error {
				e := &redskyv1alpha1.Experiment{}
				_ = k8sClient.Get(context.Background(), key, e)
				return k8sClient.Delete(context.Background(), e)
			}, timeout, interval).Should(Succeed())

			By("Expecting delete to finish")
			Eventually(func() error {
				e := &redskyv1alpha1.Experiment{}
				return k8sClient.Get(context.Background(), key, e)
			}, timeout, interval).ShouldNot(Succeed())

			_, err = redSkyAPI.GetExperimentByName(context.Background(), v1alpha1.NewExperimentName(key.Name))
			Expect(err).ShouldNot(Succeed())
		})
	})
})
