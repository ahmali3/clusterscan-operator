package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	// Import your API package so we can reference the ClusterScan struct
	scanv1alpha1 "github.com/ahmali3/clusterscan-operator/api/v1alpha1"
)

// Log for this package
var clusterscanlog = logf.Log.WithName("clusterscan-resource")

// SetupWebhookWithManager registers the webhook with the manager.
func (w *ClusterScanWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&scanv1alpha1.ClusterScan{}).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-scan-ahmali3-github-io-v1alpha1-clusterscan,mutating=true,failurePolicy=fail,sideEffects=None,groups=scan.ahmali3.github.io,resources=clusterscans,verbs=create;update,versions=v1alpha1,name=mclusterscan.kb.io,admissionReviewVersions=v1

// ClusterScanWebhook implements Defaulter and Validator
type ClusterScanWebhook struct{}

var _ webhook.CustomDefaulter = &ClusterScanWebhook{}

// Default implements webhook.CustomDefaulter (Mutating Logic)
func (w *ClusterScanWebhook) Default(ctx context.Context, obj runtime.Object) error {
	clusterscan, ok := obj.(*scanv1alpha1.ClusterScan)
	if !ok {
		return fmt.Errorf("expected an ClusterScan object but got %T", obj)
	}
	clusterscanlog.Info("Defaulting fields for ClusterScan", "name", clusterscan.Name)

	// 1. Default Command if missing
	if len(clusterscan.Spec.Command) == 0 {
		clusterscan.Spec.Command = []string{"trivy", "image", clusterscan.Spec.Image}
		clusterscanlog.Info("Defaulted command", "command", clusterscan.Spec.Command)
	}
	return nil
}

// +kubebuilder:webhook:path=/validate-scan-ahmali3-github-io-v1alpha1-clusterscan,mutating=false,failurePolicy=fail,sideEffects=None,groups=scan.ahmali3.github.io,resources=clusterscans,verbs=create;update,versions=v1alpha1,name=vclusterscan.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &ClusterScanWebhook{}

// ValidateCreate implements webhook.CustomValidator
func (w *ClusterScanWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	clusterscan, ok := obj.(*scanv1alpha1.ClusterScan)
	if !ok {
		return nil, fmt.Errorf("expected a ClusterScan object but got %T", obj)
	}
	clusterscanlog.Info("Validating create", "name", clusterscan.Name)
	return w.validateClusterScan(clusterscan)
}

// ValidateUpdate implements webhook.CustomValidator
func (w *ClusterScanWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	clusterscan, ok := newObj.(*scanv1alpha1.ClusterScan)
	if !ok {
		return nil, fmt.Errorf("expected a ClusterScan object but got %T", newObj)
	}
	clusterscanlog.Info("Validating update", "name", clusterscan.Name)
	return w.validateClusterScan(clusterscan)
}

// ValidateDelete implements webhook.CustomValidator
func (w *ClusterScanWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// Shared validation logic
func (w *ClusterScanWebhook) validateClusterScan(r *scanv1alpha1.ClusterScan) (admission.Warnings, error) {
	var warnings admission.Warnings

	// 1. Validate Image
	if r.Spec.Image == "" {
		return nil, fmt.Errorf("image cannot be empty")
	}

	// 2. Validate Schedule (Cron Syntax)
	if r.Spec.Schedule != "" {
		_, err := cron.ParseStandard(r.Spec.Schedule)
		if err != nil {
			return nil, fmt.Errorf("invalid cron schedule format: %v", err)
		}
	}

	// 3. Best Practice Warning
	if strings.HasSuffix(r.Spec.Image, ":latest") {
		warnings = append(warnings, "Warning: Using ':latest' tag is not recommended for production scanning")
	}

	return warnings, nil
}
