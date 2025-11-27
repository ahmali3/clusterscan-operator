/*
Copyright 2025.

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	scanv1alpha1 "github.com/ahmali3/clusterscan-operator/api/v1alpha1"
)

var _ = Describe("ClusterScan Webhook", func() {
	var (
		obj    *scanv1alpha1.ClusterScan
		oldObj *scanv1alpha1.ClusterScan
		// FIX: Use the correct struct name: ClusterScanWebhook
		validator ClusterScanWebhook
		defaulter ClusterScanWebhook
	)

	BeforeEach(func() {
		obj = &scanv1alpha1.ClusterScan{}
		oldObj = &scanv1alpha1.ClusterScan{}

		// FIX: Initialize the correct struct
		validator = ClusterScanWebhook{}
		defaulter = ClusterScanWebhook{}

		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	AfterEach(func() {
	})

	Context("When creating ClusterScan under Defaulting Webhook", func() {
		It("Should apply defaults when command is empty", func() {
			By("simulating a scenario where defaults should be applied")
			obj.Spec.Command = []string{} // Empty command
			obj.Spec.Image = "nginx"

			By("calling the Default method to apply defaults")
			// We pass context.Background() because our Default signature requires context
			_ = defaulter.Default(ctx, obj)

			By("checking that the default values are set")
			Expect(obj.Spec.Command).To(Equal([]string{"trivy", "image", "nginx"}))
		})
	})

	Context("When creating or updating ClusterScan under Validating Webhook", func() {
		It("Should deny creation if Image is missing", func() {
			By("simulating an invalid creation scenario")
			obj.Spec.Image = "" // Invalid

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("image cannot be empty"))
		})

		It("Should admit creation if Image is valid", func() {
			By("simulating a valid creation scenario")
			obj.Spec.Image = "nginx:1.19"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(BeNil())
		})

		It("Should warn if using latest tag", func() {
			By("simulating a risky creation scenario")
			obj.Spec.Image = "nginx:latest"

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(BeNil())         // Should succeed...
			Expect(warnings).To(HaveLen(1)) // ...but with a warning
			Expect(warnings[0]).To(ContainSubstring("not recommended"))
		})
	})
})
