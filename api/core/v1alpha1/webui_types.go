// Copyright 2022 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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

	// UpdateStrategy rolling update strategy
	// +optional
	UpdateStrategy RollingUpdateStrategy `json:"updateStrategy,omitempty"`
}

type RollingUpdateStrategy struct {
	// MaxSurge is an optional field that specifies the maximum number of Pods that
	// can be created over the desired number of Pods.
	// +optional
	MaxSurge *int32 `json:"maxSurge,omitempty"`

	// MaxUnavailable an optional field that specifies the maximum number of Pods that
	// can be unavailable during the update process.
	// +optional
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
