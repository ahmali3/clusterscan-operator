package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	scanv1alpha1 "github.com/ahmali3/clusterscan-operator/api/v1alpha1"
)

var _ = Describe("ClusterScan Controller", Ordered, func() {
	Context("When reconciling a ClusterScan", func() {
		const resourceName = "test-scan"
		const cronResourceName = "test-cron-scan"
		const namespace = "default"
		var ctx context.Context
		var cancel context.CancelFunc

		// BeforeAll runs once before all tests in this suite
		// Sets up the controller manager and starts the reconciler
		BeforeAll(func() {
			ctx, cancel = context.WithCancel(context.Background())

			// Create a manager with the test Kubernetes configuration
			mgr, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme:  scheme.Scheme,
				Metrics: metricsserver.Options{BindAddress: "0"}, // Disable metrics server
			})
			Expect(err).NotTo(HaveOccurred())

			// Create a Kubernetes clientset for direct API access
			kubeClient, err := kubernetes.NewForConfig(cfg)
			Expect(err).NotTo(HaveOccurred())

			// Initialize the reconciler with all dependencies
			reconciler := &ClusterScanReconciler{
				Client:     mgr.GetClient(),
				Scheme:     scheme.Scheme,
				Recorder:   mgr.GetEventRecorderFor("clusterscan-controller"),
				KubeClient: kubeClient,
			}
			err = reconciler.SetupWithManager(mgr)
			Expect(err).NotTo(HaveOccurred())

			// Start the manager in a goroutine (runs the controller)
			go func() {
				defer GinkgoRecover()
				err = mgr.Start(ctx)
				Expect(err).NotTo(HaveOccurred())
			}()
		})

		// AfterAll runs once after all tests complete
		// Cancels the context to stop the controller manager
		AfterAll(func() {
			cancel()
		})

		// ============================================================
		// One-Time Job Tests
		// ============================================================
		Describe("One-Time Job Lifecycle", func() {

			// Test 1: Basic Job Creation and Owner References
			// Verifies that creating a ClusterScan results in a Job being created
			// with proper owner references for automatic garbage collection
			It("should create a Job, set OwnerRef, and update Status", func() {
				// Create a minimal ClusterScan without a schedule (one-time scan)
				scan := &scanv1alpha1.ClusterScan{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: namespace},
					Spec: scanv1alpha1.ClusterScanSpec{
						Image:   "busybox",
						Command: []string{"echo", "hello"},
					},
				}
				Expect(k8sClient.Create(ctx, scan)).To(Succeed())

				createdJob := &batchv1.Job{}
				key := types.NamespacedName{Name: resourceName + "-job", Namespace: namespace}

				// Wait for the controller to create the Job
				Eventually(func() error {
					return k8sClient.Get(ctx, key, createdJob)
				}, time.Second*10, time.Millisecond*250).Should(Succeed())

				// Verify Job has correct image from ClusterScan spec
				Expect(createdJob.Spec.Template.Spec.Containers[0].Image).To(Equal("busybox"))

				// Verify owner reference is set (enables automatic cleanup)
				Expect(createdJob.OwnerReferences).To(HaveLen(1))
				Expect(createdJob.OwnerReferences[0].Kind).To(Equal("ClusterScan"))
				Expect(createdJob.OwnerReferences[0].Name).To(Equal(resourceName))

				// Verify ClusterScan status transitions to "Running"
				Eventually(func() string {
					_ = k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, scan)
					return scan.Status.Phase
				}, time.Second*10, time.Millisecond*250).Should(Equal("Running"))

				// Cleanup
				Expect(k8sClient.Delete(ctx, scan)).To(Succeed())
			})

			// Test 2: Successful Job Completion
			// Verifies that when a Job completes successfully (exit 0),
			// the ClusterScan status transitions to "Completed"
			It("should transition to Completed when Job succeeds", func() {
				scanName := "test-job-complete"
				scan := &scanv1alpha1.ClusterScan{
					ObjectMeta: metav1.ObjectMeta{Name: scanName, Namespace: namespace},
					Spec: scanv1alpha1.ClusterScanSpec{
						Image:   "busybox",
						Command: []string{"sh", "-c", "exit 0"}, // Successful command
					},
				}
				Expect(k8sClient.Create(ctx, scan)).To(Succeed())

				// Wait for Job to be created
				createdJob := &batchv1.Job{}
				jobKey := types.NamespacedName{Name: scanName + "-job", Namespace: namespace}
				Eventually(func() error {
					return k8sClient.Get(ctx, jobKey, createdJob)
				}, time.Second*10).Should(Succeed())

				// Simulate Job completion by updating its status
				Eventually(func() error {
					err := k8sClient.Get(ctx, jobKey, createdJob)
					if err != nil {
						return err
					}
					now := metav1.Now()
					createdJob.Status.Succeeded = 1
					createdJob.Status.CompletionTime = &now
					createdJob.Status.StartTime = &now
					// Set conditions that Kubernetes 1.27+ requires for completion
					createdJob.Status.Conditions = []batchv1.JobCondition{
						{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue, LastTransitionTime: now},
						{Type: batchv1.JobComplete, Status: corev1.ConditionTrue, LastTransitionTime: now},
					}
					return k8sClient.Status().Update(ctx, createdJob)
				}, time.Second*10).Should(Succeed())

				// Verify ClusterScan status reflects successful completion
				Eventually(func() string {
					_ = k8sClient.Get(ctx, types.NamespacedName{Name: scanName, Namespace: namespace}, scan)
					return scan.Status.Phase
				}, time.Second*10).Should(Equal("Completed"))

				Expect(k8sClient.Delete(ctx, scan)).To(Succeed())
			})

			// Test 3: Failed Job Handling
			// Verifies that when a Job fails (non-zero exit code),
			// the ClusterScan status transitions to "Failed"
			It("should transition to Failed when Job fails", func() {
				scanName := "test-job-failed"
				scan := &scanv1alpha1.ClusterScan{
					ObjectMeta: metav1.ObjectMeta{Name: scanName, Namespace: namespace},
					Spec: scanv1alpha1.ClusterScanSpec{
						Image:   "busybox",
						Command: []string{"exit 1"}, // Failed command
					},
				}
				Expect(k8sClient.Create(ctx, scan)).To(Succeed())

				// Wait for Job to be created
				createdJob := &batchv1.Job{}
				jobKey := types.NamespacedName{Name: scanName + "-job", Namespace: namespace}
				Eventually(func() error {
					return k8sClient.Get(ctx, jobKey, createdJob)
				}, time.Second*10).Should(Succeed())

				// Simulate Job failure by updating its status
				Eventually(func() error {
					err := k8sClient.Get(ctx, jobKey, createdJob)
					if err != nil {
						return err
					}
					now := metav1.Now()
					createdJob.Status.Failed = 1
					createdJob.Status.StartTime = &now
					// Set failure conditions
					createdJob.Status.Conditions = []batchv1.JobCondition{{
						Type: batchv1.JobFailed, Status: corev1.ConditionTrue, LastTransitionTime: now,
						Reason: "BackoffLimitExceeded", Message: "Simulated failure",
					}, {
						Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue, LastTransitionTime: now,
						Reason: "BackoffLimitExceeded", Message: "Simulated failure",
					}}
					return k8sClient.Status().Update(ctx, createdJob)
				}, time.Second*10).Should(Succeed())

				// Verify ClusterScan status reflects failure
				Eventually(func() string {
					_ = k8sClient.Get(ctx, types.NamespacedName{Name: scanName, Namespace: namespace}, scan)
					return scan.Status.Phase
				}, time.Second*10).Should(Equal("Failed"))

				Expect(k8sClient.Delete(ctx, scan)).To(Succeed())
			})
		})

		// ============================================================
		// Scheduled CronJob Tests
		// ============================================================
		Describe("Scheduled CronJob Lifecycle", func() {

			// Test 4: CronJob Management (Creation, Updates, Suspension)
			// Verifies that:
			// 1. ClusterScan with schedule creates a CronJob (not Job)
			// 2. Schedule changes are propagated to CronJob
			// 3. Suspend field controls CronJob suspension
			It("should manage CronJob creation, updates, and suspension", func() {
				// Create a ClusterScan WITH a schedule (recurring scan)
				scan := &scanv1alpha1.ClusterScan{
					ObjectMeta: metav1.ObjectMeta{Name: cronResourceName, Namespace: namespace},
					Spec: scanv1alpha1.ClusterScanSpec{
						Image:    "busybox",
						Schedule: "*/5 * * * *", // Every 5 minutes
					},
				}
				Expect(k8sClient.Create(ctx, scan)).To(Succeed())

				createdCron := &batchv1.CronJob{}
				key := types.NamespacedName{Name: cronResourceName + "-cron", Namespace: namespace}
				scanKey := types.NamespacedName{Name: cronResourceName, Namespace: namespace}

				// Wait for CronJob to be created
				Eventually(func() error {
					return k8sClient.Get(ctx, key, createdCron)
				}, time.Second*10).Should(Succeed())
				// Verify schedule matches ClusterScan spec
				Expect(createdCron.Spec.Schedule).To(Equal("*/5 * * * *"))

				// Test: Update the schedule
				By("Updating the schedule")
				Eventually(func() error {
					currentScan := &scanv1alpha1.ClusterScan{}
					if err := k8sClient.Get(ctx, scanKey, currentScan); err != nil {
						return err
					}
					currentScan.Spec.Schedule = "0 0 * * *" // Change to daily at midnight
					return k8sClient.Update(ctx, currentScan)
				}, time.Second*10).Should(Succeed())

				// Verify CronJob schedule was updated
				Eventually(func() string {
					_ = k8sClient.Get(ctx, key, createdCron)
					return createdCron.Spec.Schedule
				}, time.Second*10).Should(Equal("0 0 * * *"))

				// Test: Suspend the CronJob
				By("Suspending the CronJob")
				Eventually(func() error {
					currentScan := &scanv1alpha1.ClusterScan{}
					if err := k8sClient.Get(ctx, scanKey, currentScan); err != nil {
						return err
					}
					currentScan.Spec.Suspend = true // Suspend scanning
					return k8sClient.Update(ctx, currentScan)
				}, time.Second*10).Should(Succeed())

				// Verify CronJob is suspended
				Eventually(func() bool {
					_ = k8sClient.Get(ctx, key, createdCron)
					return createdCron.Spec.Suspend != nil && *createdCron.Spec.Suspend
				}, time.Second*10).Should(BeTrue())

				Expect(k8sClient.Delete(ctx, scan)).To(Succeed())
			})
		})
	})
})
