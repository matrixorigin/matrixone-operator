// Copyright 2024 Matrix Origin
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
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WebUISpec struct {
	PodSet `json:",inline"`

	// ServiceType is the service type of cn service
	// +optional
	// +kubebuilder:default=ClusterIP
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// UpdateStrategy rolling update strategy
	// +optional
	UpdateStrategy *RollingUpdateStrategy `json:"updateStrategy,omitempty"`

	// +optional
	ImagePullPolicy *corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
}

type WebUIDeps struct {
	// The WebUI it depends on
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	CNSet *CNSet `json:"cnset,omitempty"`
}

type WebUIStatus struct {
	ConditionalStatus `json:",inline"`
	FailoverStatus    `json:",inline"`
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

	// Spec is the desired state of WebUI
	Spec WebUISpec `json:"spec,omitempty"`

	// Deps is the dependencies of WebUI
	Deps WebUIDeps `json:"deps,omitempty"`

	Status WebUIStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WebUIList contains a list of WebUI
type WebUIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WebUI `json:"items"`
}

func (s *WebUI) GetDependencies() []recon.Dependency {
	var deps []recon.Dependency
	if s.Deps.CNSet != nil {
		deps = append(deps, &recon.ObjectDependency[*CNSet]{
			ObjectRef: s.Deps.CNSet,
			ReadyFunc: func(c *CNSet) bool {
				return recon.IsReady(&c.Status)
			},
		})
	}
	return deps
}

func (s *WebUI) GetServiceType() corev1.ServiceType {
	if s.Spec.ServiceType == "" {
		return corev1.ServiceTypeClusterIP
	}
	return s.Spec.ServiceType
}

func (s *WebUI) SetCondition(condition metav1.Condition) {
	s.Status.SetCondition(condition)
}

func (s *WebUI) GetConditions() []metav1.Condition {
	return s.Status.GetConditions()
}

func init() {
	SchemeBuilder.Register(&WebUI{}, &WebUIList{})
}
