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
// Note that MatrixOneCluster does not support specify overlay for underlying sets directly due to the size limitation
// of kubernetes apiserver
type MatrixoneClusterSpec struct {
	// TP is the default CN pod set that accepts client connections and execute queries
	// +required
	TP CNSetBasic `json:"tp"`

	// AP is an optional CN pod set that accept MPP sub-plans to accelerate sql queries
	// +optionals
	AP *CNSetBasic `json:"ap,omitempty"`

	// DN is the default DN pod set of this Cluster
	DN DNSetBasic `json:"dn"`
	// LogService is the default LogService pod set of this cluster
	LogService LogSetBasic `json:"logService"`
	// Version is the version of the cluster, which translated
	// to the docker image tag used for each component.
	// default to the recommended version of the operator
	// +required
	Version string `json:"version"`
	// ImageRepository allows user to override the default image
	// repository in order to use a docker registry proxy or private
	// registry.
	// +required
	ImageRepository string `json:"imageRepository,omitempty"`
	// WebUIEnabled indicates whether deploy the MO web-ui,
	// default to true.
	// +optional
	WebUIEnabled *bool `json:"webUIEnabled,omitempty"`
}

// MatrixoneClusterStatus defines the observed state of MatrixoneCluster
type MatrixoneClusterStatus struct {
	ConditionalStatus `json:",inline"`
	// TP is the TP set status
	TP *CNSetStatus `json:"tp,omitempty"`
	// AP is the AP set status
	AP *CNSetStatus `json:"ap,omitempty"`
	// DN is the DN set status
	DN *DNSetStatus `json:"dn,omitempty"`
	// LogService is the LogService status
	LogService *LogSetStatus `json:"logService,omitempty"`
}

// +kubebuilder:object:root=true

// A MatrixoneCluster is a resource that represents a MatrixOne Cluster
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mo
// +kubebuilder:printcolumn:name="Log",type="integer",JSONPath=".spec.logService.replicas"
// +kubebuilder:printcolumn:name="DN",type="integer",JSONPath=".spec.dn.replicas"
// +kubebuilder:printcolumn:name="TP",type="integer",JSONPath=".spec.tp.replicas"
// +kubebuilder:printcolumn:name="AP",type="integer",JSONPath=".spec.ap.replicas"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
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
