package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LogSetSpec struct {
	PodSet `json:",inline"`
	// Volume is the local persistent volume for each LogService instance
	// +required
	Volume Volume `json:"volume"`

	// SharedStorage is an external shared storage shared by all LogService instances
	// +required
	SharedStorage SharedStorageProvider `json:"sharedStorage"`

	// InitialConfig is the initial configuration of HAKeeper
	// InitialConfig is immutable
	// +optional
	InitialConfig *InitialConfig `json:"initialConfig,omitempty"`
}

type InitialConfig struct {
	// LogShards is the initial number of log shards,
	// cannot be tuned after cluster creation currently.
	// +optional
	LogShards *int `json:"logShards,omitempty"`

	// DNShards is the initial number of DN shards,
	// cannot be tuned after cluster creation currently.
	// +optional
	DNShards *int `json:"dnShards,omitempty"`
}

// TODO: figure out what status should be exposed
type LogSetStatus struct {
	ConditionalStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// A LogSet is a resource that represents a set of MO's LogService instances
// +kubebuilder:subresource:status
type LogSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LogSetSpec   `json:"spec,omitempty"`
	Status LogSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LogSetList contains a list of LogSet
type LogSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LogSet{}, &LogSetList{})
}