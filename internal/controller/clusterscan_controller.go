package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	scanv1alpha1 "github.com/ahmali3/clusterscan-operator/api/v1alpha1"
)

const (
	PhaseScheduled = "Scheduled"
	PhasePending   = "Pending"
	PhaseRunning   = "Running"
	PhaseCompleted = "Completed"
	PhaseFailed    = "Failed"
	PhaseSuspended = "Suspended"
)

type ClusterScanReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	KubeClient kubernetes.Interface
}

// +kubebuilder:rbac:groups=scan.ahmali3.github.io,resources=clusterscans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scan.ahmali3.github.io,resources=clusterscans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get

func (r *ClusterScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var clusterScan scanv1alpha1.ClusterScan
	if err := r.Get(ctx, req.NamespacedName, &clusterScan); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if clusterScan.Spec.Schedule != "" {
		return r.reconcileCronJob(ctx, &clusterScan)
	}
	return r.reconcileJob(ctx, &clusterScan)
}

func (r *ClusterScanReconciler) reconcileJob(ctx context.Context, clusterScan *scanv1alpha1.ClusterScan) (ctrl.Result, error) {
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

		clusterScan.Status.LastJobName = jobName
		clusterScan.Status.Phase = PhaseRunning
		if err := r.Status().Update(ctx, clusterScan); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if err == nil {
		condition := metav1.Condition{
			Type: "Ready", Status: metav1.ConditionFalse, Reason: "Running", Message: "Scan is in progress",
		}
		clusterScan.Status.Phase = PhaseRunning

		if job.Status.Succeeded > 0 {
			// Store scan results in ConfigMap
			if err := r.captureAndStoreScanResults(ctx, clusterScan, job); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to store results: %v", err)
			}

			condition = metav1.Condition{
				Type: "Ready", Status: metav1.ConditionTrue, Reason: "Completed", Message: "Scan completed successfully",
			}
			clusterScan.Status.LastRunTime = job.Status.CompletionTime
			clusterScan.Status.Phase = PhaseCompleted
		} else if job.Status.Failed > 0 {
			condition = metav1.Condition{
				Type: "Ready", Status: metav1.ConditionFalse, Reason: "Failed", Message: "Scan job failed",
			}
			clusterScan.Status.Phase = PhaseFailed
		}

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

func (r *ClusterScanReconciler) reconcileCronJob(ctx context.Context, clusterScan *scanv1alpha1.ClusterScan) (ctrl.Result, error) {
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

		clusterScan.Status.Phase = PhaseScheduled
		if err := r.Status().Update(ctx, clusterScan); err != nil {
			return ctrl.Result{}, err
		}
	} else if err == nil {
		currentSuspend := false
		if cronJob.Spec.Suspend != nil {
			currentSuspend = *cronJob.Spec.Suspend
		}

		if cronJob.Spec.Schedule != clusterScan.Spec.Schedule || currentSuspend != clusterScan.Spec.Suspend {
			cronJob.Spec.Schedule = clusterScan.Spec.Schedule
			cronJob.Spec.Suspend = &clusterScan.Spec.Suspend
			if err := r.Update(ctx, cronJob); err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Event(clusterScan, corev1.EventTypeNormal, "Updated", "CronJob configuration updated")
		}

		clusterScan.Status.Phase = PhaseScheduled
		if *cronJob.Spec.Suspend {
			clusterScan.Status.Phase = PhaseSuspended
		}

		if cronJob.Status.LastScheduleTime != nil {
			if clusterScan.Status.LastRunTime == nil || !cronJob.Status.LastScheduleTime.Equal(clusterScan.Status.LastRunTime) {
				clusterScan.Status.LastRunTime = cronJob.Status.LastScheduleTime
				if err := r.Status().Update(ctx, clusterScan); err != nil {
					return ctrl.Result{}, err
				}
			}
		} else if clusterScan.Status.Phase != PhaseScheduled && !clusterScan.Spec.Suspend {
			if err := r.Status().Update(ctx, clusterScan); err != nil {
				return ctrl.Result{}, err
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

func (r *ClusterScanReconciler) captureAndStoreScanResults(ctx context.Context, clusterScan *scanv1alpha1.ClusterScan, job *batchv1.Job) error {
	log := ctrl.LoggerFrom(ctx)

	podList := &corev1.PodList{}
	listOptions := []client.ListOption{
		client.InNamespace(job.Namespace),
		client.MatchingLabels{"job-name": job.Name},
	}

	if err := r.List(ctx, podList, listOptions...); err != nil {
		return fmt.Errorf("unable to list pods: %v", err)
	}

	if len(podList.Items) == 0 {
		log.Info("No pods found for completed job - skipping result storage", "job", job.Name)
		r.Recorder.Event(clusterScan, corev1.EventTypeWarning, "NoPodsFound",
			"Job completed but no pods found for result collection")
		return nil
	}

	pod := podList.Items[0]

	// Use the Shared Client here instead of creating a new one
	logRequest := r.KubeClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
	logBytes, err := logRequest.DoRaw(ctx)
	if err != nil {
		log.Error(err, "Failed to retrieve pod logs", "pod", pod.Name)
		r.Recorder.Event(clusterScan, corev1.EventTypeWarning, "LogRetrievalFailed",
			fmt.Sprintf("Could not retrieve logs from pod %s", pod.Name))
		return nil
	}

	cmName := clusterScan.Name + "-results"
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: clusterScan.Namespace,
			Labels: map[string]string{
				"app": "clusterscan",
				// This label must use the correct domain
				"scan.ahmali3.github.io/name": clusterScan.Name,
			},
		},
		Data: map[string]string{
			"scan-output.txt": string(logBytes),
			"scanner":         clusterScan.Spec.Image,
			"target":          clusterScan.Spec.Target,
			"timestamp":       time.Now().Format(time.RFC3339),
		},
	}

	if err := controllerutil.SetControllerReference(clusterScan, configMap, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %v", err)
	}

	existingCM := &corev1.ConfigMap{}
	cmKey := types.NamespacedName{Name: cmName, Namespace: clusterScan.Namespace}
	cmErr := r.Get(ctx, cmKey, existingCM)

	if cmErr != nil && errors.IsNotFound(cmErr) {
		if err := r.Create(ctx, configMap); err != nil {
			return fmt.Errorf("failed to create ConfigMap: %v", err)
		}
		r.Recorder.Event(clusterScan, corev1.EventTypeNormal, "ResultsStored",
			fmt.Sprintf("Results stored in ConfigMap %s", cmName))
	} else if cmErr == nil {
		existingCM.Data = configMap.Data
		if err := r.Update(ctx, existingCM); err != nil {
			return fmt.Errorf("failed to update ConfigMap: %v", err)
		}
	} else {
		return fmt.Errorf("error checking ConfigMap: %v", cmErr)
	}

	var exitCode int32 = 0
	if len(pod.Status.ContainerStatuses) > 0 {
		containerStatus := pod.Status.ContainerStatuses[0]
		if containerStatus.State.Terminated != nil {
			exitCode = containerStatus.State.Terminated.ExitCode
		}
	}

	clusterScan.Status.ResultsConfigMap = cmName
	clusterScan.Status.ScanExitCode = &exitCode

	return nil
}

func (r *ClusterScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scanv1alpha1.ClusterScan{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
