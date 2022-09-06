package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WebUISpec struct {
	WebUIBasic `json:",inline"`

	// +optional
	Overlay *Overlay `json:"overlay,omitempty"`
}

type WebUIBasic struct {
	PodSet `json:",inline"`

	// ServiceType is the service type of cn service
	// +optional
	// +kubebuilder:default=ClusterIP
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// RollingUpdate strategy
	// +optional
	UpdateStrategy *RollingUpdateStrategy `json:",inline"`
}

type RollingUpdateStrategy struct {
	MaxSurge       *int32 `json:"maxSurge,omitempty"`
	MaxUnavailable *int32 `json:"maxUnavailable,omitempty"`
}

type WebUIStatus struct {
	ConditionalStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// WebUI  is a resource that represents a set of MO's webui instances
// +kubebuilder:subresource:status
type WebUI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebUISpec   `json:"spec,omitempty"`
	Status WebUIStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WebUIList contains a list of WebUI
type WebUIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WebUI `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WebUI{}, &WebUIList{})
}
