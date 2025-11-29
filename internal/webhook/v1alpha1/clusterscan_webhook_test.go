package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	scanv1alpha1 "github.com/ahmali3/clusterscan-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ClusterScan Webhook", func() {
	var (
		obj       *scanv1alpha1.ClusterScan
		oldObj    *scanv1alpha1.ClusterScan
		validator ClusterScanWebhook
		defaulter ClusterScanWebhook
	)

	BeforeEach(func() {
		obj = &scanv1alpha1.ClusterScan{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-scan",
				Namespace: "default",
			},
		}
		oldObj = &scanv1alpha1.ClusterScan{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-scan",
				Namespace: "default",
			},
		}
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
		It("Should apply defaults when target is specified with Trivy", func() {
			By("simulating a Trivy scan with target")
			obj.Spec.Command = []string{}
			obj.Spec.Image = DefaultScannerImage
			obj.Spec.Target = TestTargetImage

			By("calling the Default method to apply defaults")
			err := defaulter.Default(ctx, obj)
			Expect(err).ToNot(HaveOccurred())

			By("checking that the default values are set")
			Expect(obj.Spec.Command).To(Equal([]string{"trivy", "image", TestTargetImage}))
		})

		It("Should NOT apply defaults when command is already specified", func() {
			By("simulating a custom command")
			obj.Spec.Command = []string{"custom", "command"}
			obj.Spec.Image = DefaultScannerImage
			obj.Spec.Target = TestTargetImage

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).ToNot(HaveOccurred())

			By("checking that command was not overwritten")
			Expect(obj.Spec.Command).To(Equal([]string{"custom", "command"}))
		})

		It("Should NOT apply defaults when target is empty", func() {
			By("simulating a non-image scan (e.g., kube-bench)")
			obj.Spec.Command = []string{"kube-bench", "run"}
			obj.Spec.Image = "aquasec/kube-bench:latest"
			obj.Spec.Target = ""

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).ToNot(HaveOccurred())

			By("checking that command remains unchanged")
			Expect(obj.Spec.Command).To(Equal([]string{"kube-bench", "run"}))
		})

		It("Should NOT apply defaults for non-Trivy images", func() {
			By("simulating a Grype scan")
			obj.Spec.Command = []string{}
			obj.Spec.Image = "anchore/grype:latest"
			obj.Spec.Target = TestTargetImage

			By("calling the Default method")
			err := defaulter.Default(ctx, obj)
			Expect(err).ToNot(HaveOccurred())

			By("checking that command remains empty")
			Expect(obj.Spec.Command).To(BeEmpty())
		})
	})

	Context("When creating ClusterScan under Validating Webhook", func() {
		It("Should deny creation if Image is missing", func() {
			By("simulating an invalid creation scenario")
			obj.Spec.Image = ""
			obj.Spec.Target = TestTargetImage

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("image cannot be empty"))
		})

		It("Should deny creation if both Target and Command are missing", func() {
			By("simulating invalid spec with neither target nor command")
			obj.Spec.Image = DefaultScannerImage
			obj.Spec.Target = ""
			obj.Spec.Command = []string{}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("either 'target' or 'command' must be specified"))
		})

		It("Should admit creation if Target is specified", func() {
			By("simulating a valid creation with target")
			obj.Spec.Image = DefaultScannerImage
			obj.Spec.Target = TestTargetImage

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should admit creation if Command is specified", func() {
			By("simulating a valid creation with custom command")
			obj.Spec.Image = "aquasec/kube-bench:latest"
			obj.Spec.Command = []string{"kube-bench", "run"}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should warn if using latest tag for image", func() {
			By("simulating a risky scanner image")
			obj.Spec.Image = DefaultScannerImage
			obj.Spec.Target = TestTargetImage

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(warnings).To(ContainElement(ContainSubstring("not recommended for production")))
		})

		It("Should warn if scanning latest tag", func() {
			By("simulating scanning a latest tag")
			obj.Spec.Image = "aquasec/trivy:0.48.0"
			obj.Spec.Target = "nginx:latest"

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(warnings).To(ContainElement(ContainSubstring("Scanning ':latest' tag")))
		})

		It("Should deny creation with invalid cron schedule", func() {
			By("simulating invalid cron syntax")
			obj.Spec.Image = DefaultScannerImage
			obj.Spec.Target = TestTargetImage
			obj.Spec.Schedule = "invalid cron"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid cron schedule format"))
		})

		It("Should warn about very frequent schedules", func() {
			By("simulating every-minute schedule")
			obj.Spec.Image = DefaultScannerImage
			obj.Spec.Target = TestTargetImage
			obj.Spec.Schedule = "* * * * *"

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(warnings).To(ContainElement(ContainSubstring("runs every minute")))
		})

		It("Should deny creation with uppercase in target", func() {
			By("simulating uppercase image name")
			obj.Spec.Image = DefaultScannerImage
			obj.Spec.Target = "NGINX:1.19"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must be lowercase"))
		})

		It("Should deny creation with dangerous commands", func() {
			By("simulating dangerous rm command")
			obj.Spec.Image = "alpine:latest"
			obj.Spec.Command = []string{"sh", "-c", "rm -rf /"}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("potentially dangerous operations"))
		})

		It("Should warn when both target and command are specified", func() {
			By("simulating both target and command")
			obj.Spec.Image = DefaultScannerImage
			obj.Spec.Target = TestTargetImage
			obj.Spec.Command = []string{"custom", "command"}

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(warnings).To(ContainElement(ContainSubstring("'command' will be used (target ignored)")))
		})

		It("Should warn about unknown scanner", func() {
			By("simulating non-standard scanner")
			obj.Spec.Image = "mycompany/custom-scanner:v1"
			obj.Spec.Target = TestTargetImage

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(warnings).To(ContainElement(ContainSubstring("doesn't appear to be a known security scanner")))
		})

		It("Should warn about suspend without schedule", func() {
			By("simulating suspend on one-time scan")
			obj.Spec.Image = DefaultScannerImage
			obj.Spec.Target = TestTargetImage
			obj.Spec.Suspend = true

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(warnings).To(ContainElement(ContainSubstring("suspend has no effect on one-time scans")))
		})
	})

	Context("When updating ClusterScan under Validating Webhook", func() {
		It("Should deny target change after scan completes", func() {
			By("simulating completed scan")
			oldObj.Spec.Image = DefaultScannerImage
			oldObj.Spec.Target = TestTargetImage
			oldObj.Status.Phase = "Completed"

			obj.Spec.Image = DefaultScannerImage
			obj.Spec.Target = "nginx:1.20"

			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("target is immutable after first scan completes"))
		})

		It("Should allow target change while still pending", func() {
			By("simulating pending scan")
			oldObj.Spec.Image = DefaultScannerImage
			oldObj.Spec.Target = TestTargetImage
			oldObj.Status.Phase = PhasePending

			obj.Spec.Image = DefaultScannerImage
			obj.Spec.Target = "nginx:1.20"

			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(warnings).To(ContainElement(ContainSubstring("Changing target")))
		})

		It("Should deny image change while scan is running", func() {
			By("simulating running scan")
			oldObj.Spec.Image = "aquasec/trivy:0.48.0"
			oldObj.Spec.Target = TestTargetImage
			oldObj.Status.Phase = PhaseRunning

			obj.Spec.Image = "aquasec/trivy:0.49.0"
			obj.Spec.Target = TestTargetImage

			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot change image while scan is running"))
		})

		It("Should warn about scanner type changes", func() {
			By("simulating scanner change")
			oldObj.Spec.Image = DefaultScannerImage
			oldObj.Spec.Target = TestTargetImage
			oldObj.Status.Phase = "Completed"

			obj.Spec.Image = "anchore/grype:latest"
			obj.Spec.Target = TestTargetImage

			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(warnings).To(ContainElement(ContainSubstring("Changing scanner type")))
		})

		It("Should warn about adding schedule", func() {
			By("simulating adding schedule to one-time scan")
			oldObj.Spec.Image = DefaultScannerImage
			oldObj.Spec.Target = TestTargetImage
			oldObj.Spec.Schedule = ""

			obj.Spec.Image = DefaultScannerImage
			obj.Spec.Target = TestTargetImage
			obj.Spec.Schedule = "0 2 * * *"

			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(warnings).To(ContainElement(ContainSubstring("convert from one-time to recurring scan")))
		})

		It("Should warn about removing schedule", func() {
			By("simulating removing schedule")
			oldObj.Spec.Image = DefaultScannerImage
			oldObj.Spec.Target = TestTargetImage
			oldObj.Spec.Schedule = "0 2 * * *"

			obj.Spec.Image = DefaultScannerImage
			obj.Spec.Target = TestTargetImage
			obj.Spec.Schedule = ""

			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(warnings).To(ContainElement(ContainSubstring("convert from recurring to one-time scan")))
		})
	})
})
