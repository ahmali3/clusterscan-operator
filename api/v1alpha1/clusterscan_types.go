package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterScanSpec defines the desired state of ClusterScan
type ClusterScanSpec struct {
	// +kubebuilder:validation:Required
	// Image is the container image to run (e.g., aquasec/trivy)
	Image string `json:"image"`

	// +kubebuilder:validation:Optional
	// Command allows overriding the entrypoint
	Command []string `json:"command,omitempty"`

	// +kubebuilder:validation:Optional
	// Schedule is a Cron formatted string. If omitted, the scan runs once.
	Schedule string `json:"schedule,omitempty"`

	// +kubebuilder:default=false
	// Suspend allows pausing the schedule
	Suspend bool `json:"suspend,omitempty"`
}

// ClusterScanStatus defines the observed state of ClusterScan
type ClusterScanStatus struct {
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// LastRunTime records when the job most recently completed
	LastRunTime *metav1.Time `json:"lastRunTime,omitempty"`

	// LastJobName records the name of the most recent job created
	LastJobName string `json:"lastJobName,omitempty"`

	// Phase represents the high-level status of the scan (e.g., Pending, Running, Done, Scheduled)
	// +kubebuilder:default="Pending"
	Phase string `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=`.spec.schedule`
// +kubebuilder:printcolumn:name="Last Run",type=date,JSONPath=`.status.lastRunTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ClusterScan is the Schema for the clusterscans API
type ClusterScan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterScanSpec   `json:"spec,omitempty"`
	Status ClusterScanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterScanList contains a list of ClusterScan
type ClusterScanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterScan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterScan{}, &ClusterScanList{})
}
