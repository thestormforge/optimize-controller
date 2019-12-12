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

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	"golang.org/x/net/context"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Trial Controller", func() {

	const timeout = time.Second * 30
	const interval = time.Second * 1

	Context("Empty trial", func() {
		It("Should create successfully", func() {
			key := types.NamespacedName{
				Name:      "foo",
				Namespace: "default",
			}

			instance := &redskyv1alpha1.Trial{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
			}

			Expect(k8sClient.Create(context.Background(), instance)).Should(Succeed())

			By("Expecting job to be created")
			Eventually(func() error {
				j := &batchv1.Job{}
				return k8sClient.Get(context.Background(), key, j)
			}, timeout, interval).Should(Succeed())

			// TODO Delete the job because there is no GC?

			By("Expecting to delete successfully")
			Eventually(func() error {
				t := &redskyv1alpha1.Trial{}
				_ = k8sClient.Get(context.Background(), key, t)
				return k8sClient.Delete(context.Background(), t)
			}, timeout, interval).Should(Succeed())

			By("Expecting delete to finish")
			Eventually(func() error {
				t := &redskyv1alpha1.Trial{}
				return k8sClient.Get(context.Background(), key, t)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})
})
