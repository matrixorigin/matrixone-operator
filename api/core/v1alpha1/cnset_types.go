// Copyright 2023 Matrix Origin
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

type CNRole string

const (
	CNRoleTP CNRole = "TP"
	CNRoleAP CNRole = "AP"
)

type CNSetSpec struct {
	PodSet `json:",inline"`

	// ServiceType is the service type of cn service
	// +optional
	// +kubebuilder:default=ClusterIP
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// ServiceAnnotations are the annotations for the cn service
	// +optional
	ServiceAnnotations map[string]string `json:"serviceAnnotations,omitempty"`

	// NodePort specifies the node port to use when ServiceType is NodePort or LoadBalancer,
	// reconciling will fail if the node port is not available.
	// +optional
	NodePort *int32 `json:"nodePort,omitempty"`

	// CacheVolume is the desired local cache volume for CNSet,
	// node storage will be used if not specified
	// +optional
	CacheVolume *Volume `json:"cacheVolume,omitempty"`

	// SharedStorageCache is the configuration of the S3 sharedStorageCache
	SharedStorageCache SharedStorageCache `json:"sharedStorageCache,omitempty"`

	// [TP, AP], default to TP
	// Deprecated: use labels instead
	// +optional
	Role CNRole `json:"role,omitempty"`

	// Labels are the CN labels for all the CN stores managed by this CNSet
	Labels []CNLabel `json:"cnLabels,omitempty"`
}

type CNLabel struct {
	// Key is the store label key
	Key string `json:"key,omitempty"`
	// Values are the store label values
	Values []string `json:"values,omitempty"`
}

// CNSetStatus Figure out what status should be exposed
type CNSetStatus struct {
	ConditionalStatus `json:",inline"`
}

type CNSetDeps struct {
	LogSetRef `json:",inline"`
	// The DNSet it depends on
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	DNSet *DNSet `json:"dnSet,omitempty"`
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

	// Spec is the desired state of CNSet
	Spec CNSetSpec `json:"spec"`
	// Deps is the dependencies of CNSet
	Deps CNSetDeps `json:"deps,omitempty"`

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
				return recon.IsReady(&l.Status) && recon.IsSynced(&l.Status)
			},
		})
	}
	if s.Deps.DNSet != nil {
		deps = append(deps, &recon.ObjectDependency[*DNSet]{
			ObjectRef: s.Deps.DNSet,
			ReadyFunc: func(d *DNSet) bool {
				return recon.IsReady(&d.Status) && recon.IsSynced(&d.Status)
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

func (s *CNSet) GetDNSBasedIdentity() bool {
	if s.Spec.DNSBasedIdentity == nil {
		return true
	}
	return *s.Spec.DNSBasedIdentity
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
