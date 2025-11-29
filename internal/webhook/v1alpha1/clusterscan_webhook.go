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

	scanv1alpha1 "github.com/ahmali3/clusterscan-operator/api/v1alpha1"
)

const (
	DefaultScannerImage = "aquasec/trivy:latest"
	TestTargetImage     = "nginx:1.19"
	PhasePending        = "Pending"
	PhaseRunning        = "Running"
)

var clusterscanlog = logf.Log.WithName("clusterscan-resource")

func (w *ClusterScanWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&scanv1alpha1.ClusterScan{}).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-scan-ahmali3-github-io-v1alpha1-clusterscan,mutating=true,failurePolicy=fail,sideEffects=None,groups=scan.ahmali3.github.io,resources=clusterscans,verbs=create;update,versions=v1alpha1,name=mclusterscan.kb.io,admissionReviewVersions=v1

type ClusterScanWebhook struct{}

var _ webhook.CustomDefaulter = &ClusterScanWebhook{}

func (w *ClusterScanWebhook) Default(ctx context.Context, obj runtime.Object) error {
	clusterscan, ok := obj.(*scanv1alpha1.ClusterScan)
	if !ok {
		return fmt.Errorf("expected an ClusterScan object but got %T", obj)
	}
	clusterscanlog.Info("Defaulting fields for ClusterScan", "name", clusterscan.Name)

	if clusterscan.Spec.Image == "" {
		clusterscan.Spec.Image = DefaultScannerImage
		clusterscanlog.Info("Defaulted image to trivy", "image", clusterscan.Spec.Image)
	}

	if len(clusterscan.Spec.Command) == 0 && clusterscan.Spec.Target != "" && strings.Contains(clusterscan.Spec.Image, "trivy") {
		clusterscan.Spec.Command = []string{"trivy", "image", clusterscan.Spec.Target}
		clusterscanlog.Info("Defaulted Trivy command", "command", clusterscan.Spec.Command, "target", clusterscan.Spec.Target)
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-scan-ahmali3-github-io-v1alpha1-clusterscan,mutating=false,failurePolicy=fail,sideEffects=None,groups=scan.ahmali3.github.io,resources=clusterscans,verbs=create;update,versions=v1alpha1,name=vclusterscan.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &ClusterScanWebhook{}

func (w *ClusterScanWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	clusterscan, ok := obj.(*scanv1alpha1.ClusterScan)
	if !ok {
		return nil, fmt.Errorf("expected a ClusterScan object but got %T", obj)
	}
	clusterscanlog.Info("Validating create", "name", clusterscan.Name)
	return w.validateClusterScan(clusterscan)
}

func (w *ClusterScanWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	clusterscan, ok := newObj.(*scanv1alpha1.ClusterScan)
	if !ok {
		return nil, fmt.Errorf("expected a ClusterScan object but got %T", newObj)
	}

	oldClusterScan, ok := oldObj.(*scanv1alpha1.ClusterScan)
	if !ok {
		return nil, fmt.Errorf("expected a ClusterScan object for old object but got %T", oldObj)
	}

	clusterscanlog.Info("Validating update", "name", clusterscan.Name)

	warnings, err := w.validateClusterScan(clusterscan)
	if err != nil {
		return warnings, err
	}

	updateWarnings, updateErr := w.validateClusterScanUpdate(oldClusterScan, clusterscan)
	warnings = append(warnings, updateWarnings...)
	if updateErr != nil {
		return warnings, updateErr
	}

	return warnings, nil
}

func (w *ClusterScanWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (w *ClusterScanWebhook) validateClusterScan(r *scanv1alpha1.ClusterScan) (admission.Warnings, error) {
	var warnings admission.Warnings

	if r.Spec.Image == "" {
		return nil, fmt.Errorf("image cannot be empty")
	}

	if !strings.Contains(r.Spec.Image, ":") && !strings.Contains(r.Spec.Image, "@") {
		warnings = append(warnings, "Image has no tag specified - will use 'latest' by default")
	}

	if r.Spec.Target == "" && len(r.Spec.Command) == 0 {
		return nil, fmt.Errorf("either 'target' or 'command' must be specified")
	}

	if r.Spec.Target != "" && len(r.Spec.Command) > 0 {
		warnings = append(warnings, "Both 'target' and 'command' specified - 'command' will be used (target ignored)")
	}

	if r.Spec.Schedule != "" {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		_, err := parser.Parse(r.Spec.Schedule)
		if err != nil {
			return nil, fmt.Errorf("invalid cron schedule format: %v", err)
		}

		if strings.HasPrefix(r.Spec.Schedule, "* * * * *") {
			warnings = append(warnings, "Schedule runs every minute - consider less frequent scans")
		}
	}

	if r.Spec.Target != "" {
		if err := validateImageReference(r.Spec.Target); err != nil {
			return nil, fmt.Errorf("invalid target format: %v", err)
		}
	}

	if len(r.Spec.Command) > 0 {
		cmdStr := strings.Join(r.Spec.Command, " ")
		if strings.Contains(cmdStr, "rm -rf") || strings.Contains(cmdStr, "dd if=") {
			return nil, fmt.Errorf("command contains potentially dangerous operations")
		}

		if len(r.Spec.Command) > 50 {
			warnings = append(warnings, "Command has more than 50 arguments - verify this is correct")
		}
	}

	if strings.HasSuffix(r.Spec.Image, ":latest") {
		warnings = append(warnings, "Using ':latest' tag for scanner image is not recommended for production")
	}

	if r.Spec.Target != "" && strings.HasSuffix(r.Spec.Target, ":latest") {
		warnings = append(warnings, "Scanning ':latest' tag - consider pinning to specific version for reproducibility")
	}

	knownScanners := []string{"trivy", "grype", "kube-bench", "kubesec"}
	isKnownScanner := false
	for _, scanner := range knownScanners {
		if strings.Contains(r.Spec.Image, scanner) {
			isKnownScanner = true
			break
		}
	}
	if !isKnownScanner && r.Spec.Target != "" {
		warnings = append(warnings, "Image doesn't appear to be a known security scanner (trivy, grype, kube-bench, kubesec)")
	}

	if r.Spec.Suspend && r.Spec.Schedule == "" {
		warnings = append(warnings, "'suspend' is set but no schedule is defined - suspend has no effect on one-time scans")
	}

	return warnings, nil
}

func (w *ClusterScanWebhook) validateClusterScanUpdate(old, new *scanv1alpha1.ClusterScan) (admission.Warnings, error) {
	var warnings admission.Warnings

	if old.Status.Phase != "" && old.Status.Phase != PhasePending {
		if old.Spec.Target != new.Spec.Target && old.Spec.Target != "" {
			return warnings, fmt.Errorf("target is immutable after first scan completes (current: %s, attempted: %s). Delete and recreate to scan different target",
				old.Spec.Target, new.Spec.Target)
		}
	}

	if old.Status.Phase == PhasePending && old.Spec.Target != new.Spec.Target && old.Spec.Target != "" {
		warnings = append(warnings, fmt.Sprintf("Changing target from '%s' to '%s' before first scan - ensure this is intentional",
			old.Spec.Target, new.Spec.Target))
	}

	if old.Status.Phase == PhaseRunning {
		if old.Spec.Image != new.Spec.Image {
			return warnings, fmt.Errorf("cannot change image while scan is running (wait for completion or delete the scan)")
		}
		if old.Spec.Target != new.Spec.Target {
			return warnings, fmt.Errorf("cannot change target while scan is running (wait for completion or delete the scan)")
		}
		if !equalCommands(old.Spec.Command, new.Spec.Command) {
			return warnings, fmt.Errorf("cannot change command while scan is running (wait for completion or delete the scan)")
		}
	}

	oldScannerType := detectScannerType(old.Spec.Image)
	newScannerType := detectScannerType(new.Spec.Image)
	if oldScannerType != newScannerType && oldScannerType != "unknown" {
		warnings = append(warnings, fmt.Sprintf("Changing scanner type from %s to %s - results may be incompatible",
			oldScannerType, newScannerType))
	}

	if old.Spec.Schedule != "" && new.Spec.Schedule == "" {
		warnings = append(warnings, "Removing schedule - this will convert from recurring to one-time scan")
	}
	if old.Spec.Schedule == "" && new.Spec.Schedule != "" {
		warnings = append(warnings, "Adding schedule - this will convert from one-time to recurring scan")
	}

	return warnings, nil
}

func validateImageReference(image string) error {
	if image == "" {
		return fmt.Errorf("image reference cannot be empty")
	}

	if strings.Contains(image, " ") {
		return fmt.Errorf("image reference cannot contain spaces")
	}

	if strings.Count(image, "@") > 1 {
		return fmt.Errorf("image reference has invalid digest format")
	}

	if strings.Contains(image, "@") && strings.Count(image, ":") > 1 {
		return fmt.Errorf("image reference cannot have both tag and digest")
	}

	parts := strings.Split(image, "/")
	for _, part := range parts {
		repoPart := strings.Split(strings.Split(part, ":")[0], "@")[0]
		if repoPart != strings.ToLower(repoPart) {
			return fmt.Errorf("image repository name must be lowercase")
		}
	}

	return nil
}

func detectScannerType(image string) string {
	image = strings.ToLower(image)
	switch {
	case strings.Contains(image, "trivy"):
		return "trivy"
	case strings.Contains(image, "kube-bench"):
		return "kube-bench"
	case strings.Contains(image, "grype"):
		return "grype"
	case strings.Contains(image, "kubesec"):
		return "kubesec"
	default:
		return "unknown"
	}
}

func equalCommands(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
