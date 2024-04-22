// Copyright 2024 Matrix Origin
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
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProxySetSpec struct {
	PodSet `json:",inline"`

	// ServiceType is the service type of proxy service
	// +optional
	// +kubebuilder:default=ClusterIP
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// ServiceAnnotations are the annotations for the proxy service
	// +optional
	ServiceAnnotations map[string]string `json:"serviceAnnotations,omitempty"`

	// NodePort specifies the node port to use when ServiceType is NodePort or LoadBalancer,
	// reconciling will fail if the node port is not available.
	// +optional
	NodePort *int32 `json:"nodePort,omitempty"`
}

type ProxySetStatus struct {
	ConditionalStatus `json:",inline"`

	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
}

type ProxySetDeps struct {
	LogSetRef `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// A ProxySet is a resource that represents a set of MO's Proxy instances
// +kubebuilder:subresource:status
type ProxySet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of ProxySet
	Spec ProxySetSpec `json:"spec"`
	// Deps is the dependencies of ProxySet
	Deps ProxySetDeps `json:"deps,omitempty"`

	Status ProxySetStatus `json:"status,omitempty"`
}

func (s *ProxySet) GetServiceType() corev1.ServiceType {
	if s.Spec.ServiceType == "" {
		return corev1.ServiceTypeClusterIP
	}
	return s.Spec.ServiceType
}

func (s *ProxySet) GetDependencies() []recon.Dependency {
	var deps []recon.Dependency
	if s.Deps.LogSet != nil {
		deps = append(deps, &recon.ObjectDependency[*LogSet]{
			ObjectRef: s.Deps.LogSet,
			ReadyFunc: func(l *LogSet) bool {
				return recon.IsReady(&l.Status) && recon.IsSynced(&l.Status)
			},
		})
	}
	return deps
}

func (s *ProxySet) SetCondition(condition metav1.Condition) {
	s.Status.SetCondition(condition)
}

func (s *ProxySet) GetConditions() []metav1.Condition {
	return s.Status.GetConditions()
}

//+kubebuilder:object:root=true

// ProxySetList contains a list of Proxy
type ProxySetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProxySet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProxySet{}, &ProxySetList{})
}
