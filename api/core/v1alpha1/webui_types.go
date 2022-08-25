package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type WebUISpec struct {
	WebUIBasic `json:",inline"`

	// +optional
	Overlay *Overlay `json:"overlay,omitempty"`
}

type WebUIBasic struct {
	PodSet `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// A WebUI is a resource that represents a set of MO's webui instances
// +kubebuilder:subresource:status
type WebUI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebUISpec   `json:"spec,omitempty"`
	Deps   WebUIDeps   `json:"deps,omitempty"`
	Status WebUIStatus `json:"status,omitempty"`
}

type WebUIDeps struct {
	MatrixOneClusterRef `json:",inline"`
}

type MatrixOneClusterRef struct {
	MatrixOne *MatrixOneCluster `json:"matrixone,omitempty"`
}

type WebUIStatus struct {
	ConditionalStatus `json:",inline"`
}

//+kubebuilder:object:root=true

// WebUIList contains a list of webui
type WebUIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WebUI `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WebUI{}, &WebUIList{})
}
