package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CNSetSpec struct {
	PodSet `json:",inline"`

	// CacheVolume is the desired local cache volume for CNSet,
	// node storage will be used if not specified
	// +optional
	CacheVolume *Volume `json:"cacheVolume,omitempty"`
}

// TODO: figure out what status should be exposed
type CNSetStatus struct {
	ConditionalStatus `json:",inline"`
}

type CNSetDeps struct {
	LogSetRef `json:",inline"`
}

// +kubebuilder:object:root=true

// A CNSet is a resource that represents a set of MO's CN instances
// +kubebuilder:subresource:status
type CNSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CNSetSpec   `json:"spec,omitempty"`
	Deps   CNSetDeps   `json:"deps,omitempty"`
	Status CNSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CNSetList contains a list of CNSet
type CNSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CNSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CNSet{}, &CNSetList{})
}
