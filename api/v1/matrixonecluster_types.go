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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MatrixoneClusterSpec defines the desired state of MatrixoneCluster
type MatrixoneClusterSpec struct {
	// Size                 int                        `json:"size"`
	// StartScript          string                     `json:"startScripts"`
	Image string `json:"image,omitempty"`
	// ImagePullPolicy      v1.PullPolicy              `json:"imagePullPolicy,omitempty"`
	// Env                  []v1.EnvVar                `json:"env,omitempty"`
	// ImagePullSecrets     []v1.LocalObjectReference  `json:"imagePullSecrets,omitempty"`
	// VolumeClaimTemplates []v1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
	// PodAnnotations       map[string]string          `json:"podAnnotations,omitempty"`
	// LivenessProbe        *v1.Probe                  `json:"livenessProbe,omitempty"`
	// ReadinessProbe       *v1.Probe                  `json:"readinessProbe,omitempty"`
	Services []v1.Service `json:"services,omitempty"`
	// Affinity *v1.Affinity `json:"affinity,omitempty"`
	// NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// MatrixoneClusterStatus defines the observed state of MatrixoneCluster
type MatrixoneClusterStatus struct {
	MatrixoneNodeStatus    MatrixoneNodeTypeStatus `json:"matrixoneNodeStatus,omitempty"`
	Statefulset            []string                `json:"statefulset,omitempty"`
	Deployment             []string                `json:"deployments,omitempty"`
	Services               []string                `json:"services,omitempty"`
	ConfigMaps             []string                `json:"configMaps,omitempty"`
	PodDisruptionBudgets   []string                `json:"podDisruptionBudgets,omitempty"`
	Ingress                []string                `json:"ingress,omitempty"`
	HPAutoScalers          []string                `json:"hpAutoScalers,omitempty"`
	Pods                   []string                `json:"pods,omitempty"`
	PersistentVolumeClaims []string                `json:"persistentVolumeClaims,omitempty"`
}

type MatrixoneNodeConditionType string

const (
	MatrixoneClusterReady      MatrixoneNodeConditionType = "MatrixoneClusterReady"
	MatrixoneNodeRollingUpdate MatrixoneNodeConditionType = "MatrixoneNodeRollingUpdate"
	MatrixoneNodeErrorState    MatrixoneNodeConditionType = "MatrixoneNodeErrorState"
)

type MatrixoneNodeTypeStatus struct {
	MatrixoneNode                string                     `json:"matrixoneNode,omitempty"`
	MatrixoneNodeConditionStatus v1.ConditionStatus         `json:"matrixoneNodeConditionStatus,omitempty"`
	MatrixoneNodeConditionType   MatrixoneNodeConditionType `json:"matrixoneNodeConditionType,omitempty"`
	Reason                       string                     `json:"reason,omitempty"`
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
