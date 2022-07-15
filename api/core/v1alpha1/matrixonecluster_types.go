// Copyright 2021 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MatrixoneClusterSpec defines the desired state of MatrixoneCluster
type MatrixoneClusterSpec struct {
	// CN is the default CN pod set of this Cluster
	CN CNSetSpec `json:"cn"`
	// DN is the default DN pod set of this Cluster
	DN DNSetSpec `json:"dn"`
	// LogService is the default LogService pod set of this cluster
	LogService LogSetSpec `json:"logService"`
	// Version is the version of the cluster, which translated
	// to the docker image tag used for each component.
	// default to the recommended version of the operator
	// +optional
	Version *string `json:"version"`
	// WebUIEnabled indicates whether deploy the MO web-ui,
	// default to true.
	// +optional
	WebUIEnabled *bool `json:"webUIEnabled,omitempty"`
	// ImageRepository allows user to override the default image
	// repository in order to use a docker registry proxy or private
	// registry.
	// +optional
	ImageRepository *string `json:"imageRepository,omitempty"`
}

// MatrixoneClusterStatus defines the observed state of MatrixoneCluster
type MatrixoneClusterStatus struct {
	ConditionalStatus `json:",inline"`
	// CN is the CN set status
	CN *CNSetStatus `json:"cn,omitempty"`
	// DN is the DN set status
	DN *DNSetStatus `json:"dn,omitempty"`
	// LogService is the LogService status
	LogService *LogSetStatus `json:"logService,omitempty"`
}

// +kubebuilder:object:root=true

// A MatrixoneCluster is a resource that represents a MatrixOne Cluster
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mo
type MatrixoneCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MatrixoneClusterSpec   `json:"spec,omitempty"`
	Status MatrixoneClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MatrixoneClusterList contains a list of MatrixoneCluster
type MatrixoneClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MatrixoneCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MatrixoneCluster{}, &MatrixoneClusterList{})
}
