// Copyright 2025 Matrix Origin
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DNSetSpec struct {
	PodSet `json:",inline"`

	// CacheVolume is the desired local cache volume for DNSet,
	// node storage will be used if not specified
	// +optional
	CacheVolume *Volume `json:"cacheVolume,omitempty"`

	SharedStorageCache SharedStorageCache `json:"sharedStorageCache,omitempty"`
}

type DNSetStatus struct {
	ConditionalStatus `json:",inline"`
	FailoverStatus    `json:",inline"`
}

type DNSetDeps struct {
	LogSetRef `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// A DNSet is a resource that represents a set of MO's DN instances
// +kubebuilder:subresource:status
type DNSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec is the desired state of DNSet
	Spec DNSetSpec `json:"spec"`
	// Deps is the dependencies of DNSet
	Deps DNSetDeps `json:"deps,omitempty"`

	Status DNSetStatus `json:"status,omitempty"`
}

func (d *DNSet) GetDependencies() []recon.Dependency {
	var deps []recon.Dependency
	if d.Deps.LogSet != nil {
		deps = append(deps, &recon.ObjectDependency[*LogSet]{
			ObjectRef: d.Deps.LogSet,
			ReadyFunc: func(l *LogSet) bool {
				return recon.IsReady(&l.Status) && recon.IsSyncedWithLatestGeneration(&l.Status, l.Generation)
			},
		})
	}
	return deps
}

func (d *DNSet) SetCondition(condition metav1.Condition) {
	d.Status.SetCondition(condition)
}

func (d *DNSet) GetConditions() []metav1.Condition {
	return d.Status.GetConditions()
}

func (d *DNSet) GetDNSBasedIdentity() bool {
	if d.Spec.DNSBasedIdentity == nil {
		return false
	}
	return *d.Spec.DNSBasedIdentity
}

//+kubebuilder:object:root=true

// DNSetList contains a list of DNSet
type DNSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DNSet{}, &DNSetList{})
}
