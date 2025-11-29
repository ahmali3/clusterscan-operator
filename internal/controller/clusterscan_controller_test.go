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

		BeforeAll(func() {
			ctx, cancel = context.WithCancel(context.Background())

			mgr, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme:  scheme.Scheme,
				Metrics: metricsserver.Options{BindAddress: "0"},
			})
			Expect(err).NotTo(HaveOccurred())

			kubeClient, err := kubernetes.NewForConfig(cfg)
			Expect(err).NotTo(HaveOccurred())

			reconciler := &ClusterScanReconciler{
				Client:     mgr.GetClient(),
				Scheme:     scheme.Scheme,
				Recorder:   mgr.GetEventRecorderFor("clusterscan-controller"),
				KubeClient: kubeClient,
			}
			err = reconciler.SetupWithManager(mgr)
			Expect(err).NotTo(HaveOccurred())

			go func() {
				defer GinkgoRecover()
				err = mgr.Start(ctx)
				Expect(err).NotTo(HaveOccurred())
			}()
		})

		AfterAll(func() {
			cancel()
		})

		// ============================================================
		// One-Time Job Tests
		// ============================================================
		Describe("One-Time Job Lifecycle", func() {
			It("should create a Job, set OwnerRef, and update Status", func() {
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

				Eventually(func() error {
					return k8sClient.Get(ctx, key, createdJob)
				}, time.Second*10, time.Millisecond*250).Should(Succeed())

				Expect(createdJob.Spec.Template.Spec.Containers[0].Image).To(Equal("busybox"))
				Expect(createdJob.OwnerReferences).To(HaveLen(1))
				Expect(createdJob.OwnerReferences[0].Kind).To(Equal("ClusterScan"))
				Expect(createdJob.OwnerReferences[0].Name).To(Equal(resourceName))

				Eventually(func() string {
					_ = k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, scan)
					return scan.Status.Phase
				}, time.Second*10, time.Millisecond*250).Should(Equal("Running"))

				Expect(k8sClient.Delete(ctx, scan)).To(Succeed())
			})

			It("should transition to Completed when Job succeeds", func() {
				scanName := "test-job-complete"
				scan := &scanv1alpha1.ClusterScan{
					ObjectMeta: metav1.ObjectMeta{Name: scanName, Namespace: namespace},
					Spec: scanv1alpha1.ClusterScanSpec{
						Image:   "busybox",
						Command: []string{"sh", "-c", "exit 0"},
					},
				}
				Expect(k8sClient.Create(ctx, scan)).To(Succeed())

				createdJob := &batchv1.Job{}
				jobKey := types.NamespacedName{Name: scanName + "-job", Namespace: namespace}
				Eventually(func() error {
					return k8sClient.Get(ctx, jobKey, createdJob)
				}, time.Second*10).Should(Succeed())

				Eventually(func() error {
					err := k8sClient.Get(ctx, jobKey, createdJob)
					if err != nil {
						return err
					}
					now := metav1.Now()
					createdJob.Status.Succeeded = 1
					createdJob.Status.CompletionTime = &now
					// Need these fields to simulate success for newer K8s
					createdJob.Status.StartTime = &now
					createdJob.Status.Conditions = []batchv1.JobCondition{
						{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue, LastTransitionTime: now},
						{Type: batchv1.JobComplete, Status: corev1.ConditionTrue, LastTransitionTime: now},
					}
					return k8sClient.Status().Update(ctx, createdJob)
				}, time.Second*10).Should(Succeed())

				Eventually(func() string {
					_ = k8sClient.Get(ctx, types.NamespacedName{Name: scanName, Namespace: namespace}, scan)
					return scan.Status.Phase
				}, time.Second*10).Should(Equal("Completed"))

				Expect(k8sClient.Delete(ctx, scan)).To(Succeed())
			})

			It("should transition to Failed when Job fails", func() {
				scanName := "test-job-failed"
				scan := &scanv1alpha1.ClusterScan{
					ObjectMeta: metav1.ObjectMeta{Name: scanName, Namespace: namespace},
					Spec: scanv1alpha1.ClusterScanSpec{
						Image:   "busybox",
						Command: []string{"exit 1"},
					},
				}
				Expect(k8sClient.Create(ctx, scan)).To(Succeed())

				createdJob := &batchv1.Job{}
				jobKey := types.NamespacedName{Name: scanName + "-job", Namespace: namespace}
				Eventually(func() error {
					return k8sClient.Get(ctx, jobKey, createdJob)
				}, time.Second*10).Should(Succeed())

				Eventually(func() error {
					err := k8sClient.Get(ctx, jobKey, createdJob)
					if err != nil {
						return err
					}
					now := metav1.Now()
					createdJob.Status.Failed = 1
					createdJob.Status.StartTime = &now
					createdJob.Status.Conditions = []batchv1.JobCondition{{
						Type: batchv1.JobFailed, Status: corev1.ConditionTrue, LastTransitionTime: now,
						Reason: "BackoffLimitExceeded", Message: "Simulated failure",
					}, {
						Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue, LastTransitionTime: now,
						Reason: "BackoffLimitExceeded", Message: "Simulated failure",
					}}
					return k8sClient.Status().Update(ctx, createdJob)
				}, time.Second*10).Should(Succeed())

				Eventually(func() string {
					_ = k8sClient.Get(ctx, types.NamespacedName{Name: scanName, Namespace: namespace}, scan)
					return scan.Status.Phase
				}, time.Second*10).Should(Equal("Failed"))

				Expect(k8sClient.Delete(ctx, scan)).To(Succeed())
			})
		})

		Describe("Scheduled CronJob Lifecycle", func() {
			It("should manage CronJob creation, updates, and suspension", func() {
				scan := &scanv1alpha1.ClusterScan{
					ObjectMeta: metav1.ObjectMeta{Name: cronResourceName, Namespace: namespace},
					Spec: scanv1alpha1.ClusterScanSpec{
						Image:    "busybox",
						Schedule: "*/5 * * * *",
					},
				}
				Expect(k8sClient.Create(ctx, scan)).To(Succeed())

				createdCron := &batchv1.CronJob{}
				key := types.NamespacedName{Name: cronResourceName + "-cron", Namespace: namespace}
				scanKey := types.NamespacedName{Name: cronResourceName, Namespace: namespace}

				Eventually(func() error {
					return k8sClient.Get(ctx, key, createdCron)
				}, time.Second*10).Should(Succeed())
				Expect(createdCron.Spec.Schedule).To(Equal("*/5 * * * *"))

				By("Updating the schedule")
				Eventually(func() error {
					currentScan := &scanv1alpha1.ClusterScan{}
					if err := k8sClient.Get(ctx, scanKey, currentScan); err != nil {
						return err
					}
					currentScan.Spec.Schedule = "0 0 * * *"
					return k8sClient.Update(ctx, currentScan)
				}, time.Second*10).Should(Succeed())

				Eventually(func() string {
					_ = k8sClient.Get(ctx, key, createdCron)
					return createdCron.Spec.Schedule
				}, time.Second*10).Should(Equal("0 0 * * *"))

				By("Suspending the CronJob")
				Eventually(func() error {
					currentScan := &scanv1alpha1.ClusterScan{}
					if err := k8sClient.Get(ctx, scanKey, currentScan); err != nil {
						return err
					}
					currentScan.Spec.Suspend = true
					return k8sClient.Update(ctx, currentScan)
				}, time.Second*10).Should(Succeed())

				Eventually(func() bool {
					_ = k8sClient.Get(ctx, key, createdCron)
					return createdCron.Spec.Suspend != nil && *createdCron.Spec.Suspend
				}, time.Second*10).Should(BeTrue())

				Expect(k8sClient.Delete(ctx, scan)).To(Succeed())
			})
		})
	})
})
