// Copyright 2025 Matrix Origin
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

type CNClaimPhase string

const (
	CNClaimPhasePending CNClaimPhase = "Pending"
	CNClaimPhaseBound   CNClaimPhase = "Bound"
	CNClaimPhaseLost    CNClaimPhase = "Lost"

	CNClaimPhaseOutdated CNClaimPhase = "Outdated"

	PodOwnerNameLabel = "matrixorigin.io/owner"

	ClaimSetNameLabel = "matrixorigin.io/claimset"

	DeleteOnReclaimAnno = "matrixorigin.io/delete-on-reclaim"

	// PodLastOwnerLabel records the last owner of the pod
	PodLastOwnerLabel = "matrixorigin.io/last-owner"

	// PodOutdatedLabel denotes the pod is outdated and should not be bound
	PodOutdatedLabel = "matrixorigin.io/outdated"
)

type CNClaimSpec struct {
	ClaimPodRef `json:",inline"`

	// sourcePod is the pod that previously owned by this claim and is now being migrated
	SourcePod *ClaimPodRef `json:"sourcePod,omitempty"`

	Selector *metav1.LabelSelector `json:"selector"`

	// +optional
	CNLabels []CNLabel `json:"cnLabels,omitempty"`

	// +optional
	OwnerName *string `json:"ownerName,omitempty"`

	// +optional
	// AdditionalPodLabels specifies the addition labels added to Pod after the Pod is claimed by this claim
	AdditionalPodLabels map[string]string `json:"additionalPodLabels,omitempty"`

	// +optional
	// PoolName is usually populated by controller that which pool the claim is nominated
	PoolName string `json:"poolName,omitempty"`
}

type ClaimPodRef struct {
	// +optional
	// PodName is usually populated by controller and would be part of the claim spec
	// that must be persisted once bound
	PodName string `json:"podName,omitempty"`

	// +optional
	// NodeName is usually populated by controller and would be part of the claim spec
	NodeName string `json:"nodeName,omitempty"`
}

type CNClaimStatus struct {
	Phase CNClaimPhase  `json:"phase,omitempty"`
	Store CNStoreStatus `json:"store,omitempty"`

	BoundTime *metav1.Time `json:"boundTime,omitempty"`

	// migrate is the migrating status of Pods under CNClaim
	Migrate *MigrateStatus `json:"migrate,omitempty"`
}

type CNStoreStatus struct {
	ServiceID              string    `json:"serviceID,omitempty"`
	LockServiceAddress     string    `json:"lockServiceAddress,omitempty"`
	PipelineServiceAddress string    `json:"pipelineServiceAddress,omitempty"`
	SQLAddress             string    `json:"sqlAddress,omitempty"`
	QueryAddress           string    `json:"queryAddress,omitempty"`
	WorkState              int32     `json:"workState,omitempty"`
	Labels                 []CNLabel `json:"labels,omitempty"`

	// PodName is the CN PodName
	PodName string `json:"string,omitempty"`
	// BoundTime is the time when the CN is bound
	BoundTime *metav1.Time `json:"boundTime,omitempty"`
}

type MigrateStatus struct {
	Source Workload `json:"source,omitempty"`
}

type Workload struct {
	Connections int `json:"connections,omitempty"`
	Pipelines   int `json:"pipelines,omitempty"`
	// Replicas is the sum of sharding tables served on the current CN
	Replicas int `json:"replicas,omitempty"`
}

// +kubebuilder:object:root=true

// CNClaim claim a CN to use
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Pod",type="string",JSONPath=".spec.podName"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope="Namespaced"
type CNClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CNClaimSpec `json:"spec,omitempty"`

	// +optional
	Status CNClaimStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CNClaimList contains a list of CNClaims
type CNClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CNClaim `json:"items"`
}

type CNClaimSetSpec struct {
	Replicas int32           `json:"replicas"`
	Template CNClaimTemplate `json:"template"`

	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

type CNClaimTemplate struct {
	EmbeddedMetadata `json:"metadata,omitempty"`

	Spec CNClaimSpec `json:"spec,omitempty"`
}

type EmbeddedMetadata struct {
	// +optional
	Name string `json:"name,omitempty"`

	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

type CNClaimSetStatus struct {
	Replicas      int32           `json:"replicas"`
	ReadyReplicas int32           `json:"readyReplicas"`
	Claims        []CNClaimStatus `json:"claims,omitempty"`
	LabelSelector string          `json:"labelSelector,omitempty"`

	// +optional
	// deprecated
	PodSelector string `json:"podSelector,omitempty"`
}

// +kubebuilder:object:root=true

// CNClaimSet orchestrates a set of CNClaims
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope="Namespaced"
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.labelSelector
type CNClaimSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CNClaimSetSpec `json:"spec,omitempty"`

	// +optional
	Status CNClaimSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CNClaimSetList contains a list of CNClaimSet
type CNClaimSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CNClaimSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CNClaim{}, &CNClaimList{})
	SchemeBuilder.Register(&CNClaimSet{}, &CNClaimSetList{})
}
