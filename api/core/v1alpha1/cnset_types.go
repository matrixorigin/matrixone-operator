// Copyright 2022 Matrix Origin
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
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CNSetSpec struct {
	CNSetBasic `json:",inline"`

	// +optional
	Overlay *Overlay `json:"overlay,omitempty"`
}

type CNSetBasic struct {
	PodSet `json:",inline"`

	// ServiceType is the service type of cn service
	// +optional
	// +kubebuilder:default=ClusterIP
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// CacheVolume is the desired local cache volume for CNSet,
	// node storage will be used if not specified
	// +optional
	CacheVolume *Volume `json:"cacheVolume,omitempty"`
}

type CNStore struct {
	PodName            string      `json:"podName,omitempty"`
	Phase              string      `json:"phase,omitempty"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// TODO: figure out what status should be exposed
type CNSetStatus struct {
	AvailableStores   []CNStore `json:"availableStores,omitempty"`
	FailedStores      []CNStore `json:"failedStores,omitempty"`
	ConditionalStatus `json:",inline"`
}

type CNSetDeps struct {
	LogSetRef `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// A CNSet is a resource that represents a set of MO's CN instances
// +kubebuilder:subresource:status
type CNSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CNSetSpec   `json:"spec,omitempty"`
	Deps   CNSetDeps   `json:"deps,omitempty"`
	Status CNSetStatus `json:"status,omitempty"`
}

func (s *CNSet) GetServiceType() corev1.ServiceType {
	if s.Spec.ServiceType == "" {
		return corev1.ServiceTypeClusterIP
	}
	return s.Spec.ServiceType
}

func (s *CNSet) GetDependencies() []recon.Dependency {
	var deps []recon.Dependency
	if s.Deps.LogSet != nil {
		deps = append(deps, &recon.ObjectDependency[*LogSet]{
			ObjectRef: s.Deps.LogSet,
			ReadyFunc: func(l *LogSet) bool {
				return recon.IsReady(&l.Status)
			},
		})
	}
	return deps
}

func (s *CNSet) SetCondition(condition metav1.Condition) {
	s.Status.SetCondition(condition)
}

func (s *CNSet) GetConditions() []metav1.Condition {
	return s.Status.GetConditions()
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
