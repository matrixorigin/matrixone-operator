/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MatrixoneClusterSpec defines the desired state of MatrixoneCluster
type MatrixoneClusterSpec struct {
	Replicas           int32                       `json:"replicas,omitempty"`
	Image              string                      `json:"image,omitempty"`
	MOPort             int32                       `json:"moPort,omitempty"`
	Services           v1.Service                  `json:"services,omitempty"`
	StorageClass       string                      `json:"storage-class,omitempty"`
	LogVolResource     corev1.ResourceRequirements `json:"log-volume-resource,omitempty"`
	DataVolResource    corev1.ResourceRequirements `json:"data-volume-resource,omitempty"`
	ConfigMap          v1.ConfigMap                `json:"configmap,omitempty"`
	MetircAddr         string                      `json:"metric-addr,omitemtpy"`
	ShardCapacityBytes string                      `json:"shard-capacity-bytes"`
	LowSpaceRatio      float32                     `json:"low-space-ratio"`
	HighSpaceRation    float32                     `json:"high-space-ratio"`
	MaxReplicas        int                         `json:"max-replicas"`
	StorePath          string                      `json:"store-path"`
	MaxEntryBytes      string                      `json:"max-entry-bytes"`
	Env                []corev1.EnvVar             `json:"env,omitempty"`
}

// MatrixoneClusterStatus defines the observed state of MatrixoneCluster
type MatrixoneClusterStatus struct {
	SSStatus      appsv1.StatefulSetStatus `json:"statefulsetstatus,omitempty"`
	ServiceStatus v1.ServiceStatus         `json:"servicvestatus,omitempty"`
}

// Matrixone Log with promtail and loki
type PromtailLokiSpec struct {
}

// Matrixone Monitor with prometus
type PrometheusSpec struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MatrixoneCluster is the Schema for the matrixoneclusters API
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
