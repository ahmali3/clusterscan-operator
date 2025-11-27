package controller

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	scanv1alpha1 "github.com/ahmali3/clusterscan-operator/api/v1alpha1"
)

type ClusterScanReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=scan.ahmali3.github.io,resources=clusterscans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scan.ahmali3.github.io,resources=clusterscans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *ClusterScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var clusterScan scanv1alpha1.ClusterScan
	if err := r.Get(ctx, req.NamespacedName, &clusterScan); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if clusterScan.Spec.Schedule != "" {
		return r.reconcileCronJob(ctx, &clusterScan, logger)
	}
	return r.reconcileJob(ctx, &clusterScan, logger)
}

func (r *ClusterScanReconciler) reconcileJob(ctx context.Context, clusterScan *scanv1alpha1.ClusterScan, log interface{}) (ctrl.Result, error) {
	jobName := clusterScan.Name + "-job"
	job := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: clusterScan.Namespace}, job)

	if err != nil && errors.IsNotFound(err) {
		desiredJob := r.constructJob(clusterScan, jobName)
		if err := controllerutil.SetControllerReference(clusterScan, desiredJob, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desiredJob); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Event(clusterScan, corev1.EventTypeNormal, "JobCreated", "One-off scan job created")

		// Update Status to Running immediately
		clusterScan.Status.LastJobName = jobName
		clusterScan.Status.Phase = "Running"
		if err := r.Status().Update(ctx, clusterScan); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if err == nil {
		// Default condition
		condition := metav1.Condition{
			Type: "Ready", Status: metav1.ConditionFalse, Reason: "Running", Message: "Scan is in progress",
		}
		clusterScan.Status.Phase = "Running"

		// Check for success/failure
		if job.Status.Succeeded > 0 {
			condition = metav1.Condition{
				Type: "Ready", Status: metav1.ConditionTrue, Reason: "Completed", Message: "Scan completed successfully",
			}
			clusterScan.Status.LastRunTime = job.Status.CompletionTime
			clusterScan.Status.Phase = "Completed"
		} else if job.Status.Failed > 0 {
			condition = metav1.Condition{
				Type: "Ready", Status: metav1.ConditionFalse, Reason: "Failed", Message: "Scan job failed",
			}
			clusterScan.Status.Phase = "Failed"
		}

		// Ensure Job Name is tracked
		if clusterScan.Status.LastJobName != jobName {
			clusterScan.Status.LastJobName = jobName
		}

		meta.SetStatusCondition(&clusterScan.Status.Conditions, condition)
		if err := r.Status().Update(ctx, clusterScan); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *ClusterScanReconciler) reconcileCronJob(ctx context.Context, clusterScan *scanv1alpha1.ClusterScan, log interface{}) (ctrl.Result, error) {
	cronName := clusterScan.Name + "-cron"
	cronJob := &batchv1.CronJob{}
	err := r.Get(ctx, types.NamespacedName{Name: cronName, Namespace: clusterScan.Namespace}, cronJob)

	desiredCron := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: cronName, Namespace: clusterScan.Namespace},
		Spec: batchv1.CronJobSpec{
			Schedule: clusterScan.Spec.Schedule,
			Suspend:  &clusterScan.Spec.Suspend,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{{
								Name:    "scanner",
								Image:   clusterScan.Spec.Image,
								Command: clusterScan.Spec.Command,
							}},
						},
					},
				},
			},
		},
	}

	if err != nil && errors.IsNotFound(err) {
		if err := controllerutil.SetControllerReference(clusterScan, desiredCron, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desiredCron); err != nil {
			return ctrl.Result{}, err
		}
		r.Recorder.Eventf(clusterScan, corev1.EventTypeNormal, "Scheduled", "CronJob created: %s", clusterScan.Spec.Schedule)

		clusterScan.Status.Phase = "Scheduled"
		if err := r.Status().Update(ctx, clusterScan); err != nil {
			return ctrl.Result{}, err
		}
	} else if err == nil {
		if cronJob.Spec.Schedule != clusterScan.Spec.Schedule {
			cronJob.Spec.Schedule = clusterScan.Spec.Schedule
			if err := r.Update(ctx, cronJob); err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Event(clusterScan, corev1.EventTypeNormal, "Updated", "Schedule updated")
		}

		// Update Status
		clusterScan.Status.Phase = "Scheduled"
		if cronJob.Status.LastScheduleTime != nil {
			if clusterScan.Status.LastRunTime == nil || !cronJob.Status.LastScheduleTime.Equal(clusterScan.Status.LastRunTime) {
				clusterScan.Status.LastRunTime = cronJob.Status.LastScheduleTime
				if err := r.Status().Update(ctx, clusterScan); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}
	return ctrl.Result{}, nil
}

func (r *ClusterScanReconciler) constructJob(clusterScan *scanv1alpha1.ClusterScan, name string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: clusterScan.Namespace},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{{
						Name:    "scanner",
						Image:   clusterScan.Spec.Image,
						Command: clusterScan.Spec.Command,
					}},
				},
			},
		},
	}
}

func (r *ClusterScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scanv1alpha1.ClusterScan{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
