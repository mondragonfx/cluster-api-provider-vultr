/*
Copyright 2024.

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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrastructurev1beta2 "github.com/vultr/cluster-api-provider-vultr/api/v1beta2"
)

var _ = Describe("VultrBareMetalMachine Controller", func() {
	ctx := context.Background()

	Context("When reconciling a resource", func() {
		const resourceName = "test-bare-metal"

		typeNamespacedName := types.NamespacedName{Name: resourceName, Namespace: testNamespace}

		BeforeEach(func() {
			By("creating the custom resource for the Kind VultrBareMetalMachine")
			existing := &infrastructurev1beta2.VultrBareMetalMachine{}
			err := k8sClient.Get(ctx, typeNamespacedName, existing)
			if err != nil && errors.IsNotFound(err) {
				resource := &infrastructurev1beta2.VultrBareMetalMachine{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: testNamespace},
					Spec: infrastructurev1beta2.VultrBareMetalMachineSpec{
						Region: testRegion,
						PlanID: testBareMetalPlan,
						OSID:   2284,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &infrastructurev1beta2.VultrBareMetalMachine{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
			By("Cleanup the specific resource instance VultrBareMetalMachine")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &VultrBareMetalMachineReconciler{Client: k8sClient}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When validating the spec", func() {
		newMachine := func(name string, spec infrastructurev1beta2.VultrBareMetalMachineSpec) *infrastructurev1beta2.VultrBareMetalMachine {
			return &infrastructurev1beta2.VultrBareMetalMachine{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
				Spec:       spec,
			}
		}

		It("rejects a spec without osID", func() {
			m := newMachine("no-os", infrastructurev1beta2.VultrBareMetalMachineSpec{Region: testRegion, PlanID: testBareMetalPlan})
			err := k8sClient.Create(ctx, m)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
		})

		It("accepts a valid spec and rejects an invalid disk mode", func() {
			ok := newMachine("valid", infrastructurev1beta2.VultrBareMetalMachineSpec{Region: testRegion, PlanID: testBareMetalPlan, OSID: 2284})
			Expect(k8sClient.Create(ctx, ok)).To(Succeed())
			Expect(k8sClient.Delete(ctx, ok)).To(Succeed())

			bad := newMachine("disk", infrastructurev1beta2.VultrBareMetalMachineSpec{Region: testRegion, PlanID: testBareMetalPlan, OSID: 2284, MdiskMode: "raid5"})
			Expect(k8sClient.Create(ctx, bad)).NotTo(Succeed())
		})
	})
})
