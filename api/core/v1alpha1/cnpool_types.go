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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math"
)

const (
	CNPodPhaseBound       = "Bound"
	CNPodPhaseIdle        = "Idle"
	CNPodPhaseDraining    = "Draining"
	CNPodPhaseUnknown     = "Unknown"
	CNPodPhaseTerminating = "Terminating"
)

const (
	PodManagementPolicyPooling = "Pooling"
)

const (
	// PodClaimedByLabel is a Pod label records the claim the claims the Pod
	PodClaimedByLabel = "pool.matrixorigin.io/claimed-by"
	// CNPodPhaseLabel is the pod phase in Pool
	CNPodPhaseLabel = "pool.matrixorigin.io/phase"
	// PoolNameLabel is the pool of CN claim or CN Pod
	PoolNameLabel = "pool.matrixorigin.io/pool-name"

	// PodManagementPolicyAnno denotes the management policy of a Pod
	PodManagementPolicyAnno = "pool.matrixorigin.io/management-policy"
)

type CNPoolSpec struct {
	// Template is the CNSet template of the Pool
	Template CNSetSpec `json:"template"`

	// PodLabels is the Pod labels of the CN in Pool
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// Deps is the dependencies of the Pool
	Deps CNSetDeps `json:"deps"`

	Strategy PoolStrategy `json:"strategy"`
}

type PoolStrategy struct {
	// UpdateStrategy defines the strategy for pool updating
	UpdateStrategy PoolUpdateStrategy `json:"updateStrategy"`

	// UpdateStrategy defines the strategy for pool scaling
	ScaleStrategy PoolScaleStrategy `json:"scaleStrategy"`
}

type PoolUpdateStrategy struct {
	// +optional
	ReclaimTimeout *metav1.Duration `json:"reclaimTimeout,omitempty"`
}

type PoolScaleStrategy struct {
	MaxIdle int32 `json:"maxIdle"`

	// +optional
	// MaxPods allowed in this Pool, nil means no limit
	MaxPods *int32 `json:"maxPods,omitempty"`
}

func (s *PoolScaleStrategy) GetMaxPods() int32 {
	if s.MaxPods == nil {
		return math.MaxInt32
	}
	return *s.MaxPods
}

type CNPoolStatus struct {
}

// +kubebuilder:object:root=true

// CNPool maintains a pool of CN Pods
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope="Namespaced"
type CNPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CNPoolSpec `json:"spec"`

	// +optional
	Status CNPoolStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CNPoolList contains a list of CNPool
type CNPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CNPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CNPool{}, &CNPoolList{})
}
