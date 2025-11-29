package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

		// Runs ONCE before all tests in this Context
		BeforeAll(func() {
			ctx, cancel = context.WithCancel(context.Background())

			// Disable metrics to prevent port conflicts
			mgr, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme:  scheme.Scheme,
				Metrics: metricsserver.Options{BindAddress: "0"},
			})
			Expect(err).NotTo(HaveOccurred())

			reconciler := &ClusterScanReconciler{
				Client: mgr.GetClient(),
				Scheme: scheme.Scheme,
				// Use the real recorder so events are stored in the API server
				Recorder: mgr.GetEventRecorderFor("clusterscan-controller"),
			}
			err = reconciler.SetupWithManager(mgr)
			Expect(err).NotTo(HaveOccurred())

			go func() {
				defer GinkgoRecover()
				err = mgr.Start(ctx)
				Expect(err).NotTo(HaveOccurred())
			}()
		})

		// Runs ONCE after all tests in this Context
		AfterAll(func() {
			cancel()
		})

		It("should create a One-Off Job and set Phase to Running", func() {
			scan := &scanv1alpha1.ClusterScan{
				ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: namespace},
				Spec: scanv1alpha1.ClusterScanSpec{
					Image: "busybox", Command: []string{"echo", "hello"},
				},
			}
			Expect(k8sClient.Create(ctx, scan)).To(Succeed())

			createdJob := &batchv1.Job{}
			key := types.NamespacedName{Name: resourceName + "-job", Namespace: namespace}

			// Wait for Job creation
			Eventually(func() error {
				return k8sClient.Get(ctx, key, createdJob)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			// Verify Status Phase
			Eventually(func() string {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, scan)
				return scan.Status.Phase
			}, time.Second*10, time.Millisecond*250).Should(Equal("Running"))

			Expect(k8sClient.Delete(ctx, scan)).To(Succeed())
		})

		It("should create and update a CronJob", func() {
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

			// 1. Verify CronJob creation
			Eventually(func() error {
				return k8sClient.Get(ctx, key, createdCron)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			// Verify Phase
			Eventually(func() string {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: cronResourceName, Namespace: namespace}, scan)
				return scan.Status.Phase
			}, time.Second*10, time.Millisecond*250).Should(Equal("Scheduled"))

			// 2. Verify Update Logic (Change schedule)
			// We must fetch the latest version before updating to avoid conflict
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: cronResourceName, Namespace: namespace}, scan)
			}, time.Second*5).Should(Succeed())

			scan.Spec.Schedule = "0 0 * * *"
			Expect(k8sClient.Update(ctx, scan)).To(Succeed())

			Eventually(func() string {
				_ = k8sClient.Get(ctx, key, createdCron)
				return createdCron.Spec.Schedule
			}, time.Second*10, time.Millisecond*250).Should(Equal("0 0 * * *"))

			Expect(k8sClient.Delete(ctx, scan)).To(Succeed())
		})
	})
})
