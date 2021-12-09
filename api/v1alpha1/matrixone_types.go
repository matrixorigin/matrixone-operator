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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MatrixoneSpec defines the desired state of Matrixone
type MatrixoneSpec struct {
	KubernetesConfig  KubernetesConfig           `json:"kubernetesConfig"`
	MatrixoneExporter *MatrixoneExporter         `json:"matrixoneExporter,omitempty"`
	MatrixoneConfig   *MatrixoneConfig           `json:"matrixoneConfig,omitempty"`
	Storage           *Stroage                   `json:"storage.omitempty"`
	NodeSelector      map[string]string          `json:"nodeSelector,omitempty"`
	SecurityContext   *corev1.PodSecurityContext `json:"securityContext,omitempty"`
	PriorityClassName string                     `json:"priorityClassName,omitempty"`
	Affinity          *corev1.Affinity           `json:"affinity,omitempty"`
	Tolerations       *[]corev1.Toleration       `json:"tolerations,omitempty"`
	ReadinessProbe    *corev1.Probe              `json:"readinessProbe,omitempty" protobuf:"bytes,11,opt,name=readinessProbe"`
	LivenessProbe     *corev1.Probe              `json:"livenessProbe,omitempty" protobuf:"bytes,11,opt,name=livenessProbe"`
}

// MatrixoneStatus defines the observed state of Matrixone
type MatrixoneStatus struct {
	Matrixone MatrixoneSpec `json:"matrixone,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Matrixone is the Schema for the matrixones API
type Matrixone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MatrixoneSpec   `json:"spec,omitempty"`
	Status MatrixoneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MatrixoneList contains a list of Matrixone
type MatrixoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Matrixone `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Matrixone{}, &MatrixoneList{})
}