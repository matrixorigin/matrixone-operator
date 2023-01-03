// Copyright 2023 Matrix Origin
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
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	StorePhaseUp   = "Up"
	StorePhaseDown = "Down"
)

type LogSetSpec struct {
	LogSetBasic `json:",inline"`

	// +optional
	Overlay *Overlay `json:"overlay,omitempty"`
}

type LogSetBasic struct {
	PodSet `json:",inline"`

	// Volume is the local persistent volume for each LogService instance
	// +required
	Volume Volume `json:"volume"`

	// SharedStorage is an external shared storage shared by all LogService instances
	// +required
	SharedStorage SharedStorageProvider `json:"sharedStorage"`

	// InitialConfig is the initial configuration of HAKeeper
	// InitialConfig is immutable
	// +optional
	InitialConfig InitialConfig `json:"initialConfig"`

	// StoreFailureTimeout is the timeout to fail-over the logset Pod after a failure of it is observed
	// +optional
	StoreFailureTimeout *metav1.Duration `json:"storeFailureTimeout,omitempty"`

	// PVCRetentionPolicy defines the retention policy of orphaned PVCs due to cluster deletion, scale-in
	// or failover. Available options:
	// - Delete: delete orphaned PVCs
	// - Retain: keep orphaned PVCs, if the corresponding Pod get created again (e.g. scale-in and scale-out, recreate the cluster),
	// the Pod will reuse the retained PVC which contains previous data. Retained PVCs require manual cleanup if they are no longer needed.
	// The default policy is Delete.
	// +optional
	PVCRetentionPolicy *PVCRetentionPolicy `json:"pvcRetentionPolicy,omitempty"`
}

func (l *LogSetBasic) GetStoreFailureTimeout() metav1.Duration {
	if l.StoreFailureTimeout == nil {
		return metav1.Duration{Duration: defaultStoreFailureTimeout}
	}
	return *l.StoreFailureTimeout
}

func (l *LogSetBasic) GetPVCRetentionPolicy() PVCRetentionPolicy {
	if l.PVCRetentionPolicy == nil {
		return PVCRetentionPolicyDelete
	}
	return *l.PVCRetentionPolicy
}

type InitialConfig struct {
	// LogShards is the initial number of log shards,
	// cannot be tuned after cluster creation currently.
	// default to 1
	// +required
	LogShards *int `json:"logShards,omitempty"`

	// DNShards is the initial number of DN shards,
	// cannot be tuned after cluster creation currently.
	// default to 1
	// +required
	DNShards *int `json:"dnShards,omitempty"`

	// HAKeeperReplicas is the initial number of HAKeeper replicas,
	// cannot be tuned after cluster creation currently.
	// default to 3 if LogSet replicas >= 3, to 1 otherwise
	// +required
	// HAKeeperReplicas *int `json:"haKeeperReplicas,omitempty"`

	// LogShardReplicas is the replica numbers of each log shard,
	// cannot be tuned after cluster creation currently.
	// default to 3 if LogSet replicas >= 3, to 1 otherwise
	// +required
	LogShardReplicas *int `json:"logShardReplicas,omitempty"`
}

// TODO: figure out what status should be exposed
type LogSetStatus struct {
	ConditionalStatus `json:",inline"`
	FailoverStatus    `json:",inline"`

	Discovery *LogSetDiscovery `json:"discovery,omitempty"`
	// TODO(aylei): collect LogShards, DNShards and HAKeeper status from HAKeeper
	// HAKeeper          *HAKeeperStatus  `json:"haKeeper,omitempty"`
	// LogShards
	// DNShards
}

type LogSetDiscovery struct {
	Port    int32  `json:"port,omitempty"`
	Address string `json:"address,omitempty"`
}

func (l *LogSetDiscovery) String() string {
	return fmt.Sprintf("%s:%d", l.Address, l.Port)
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// A LogSet is a resource that represents a set of MO's LogService instances
// +kubebuilder:subresource:status
type LogSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of LogSet
	Spec LogSetSpec `json:"spec"`

	Status LogSetStatus `json:"status,omitempty"`
}

func (d *LogSet) SetCondition(condition metav1.Condition) {
	d.Status.SetCondition(condition)
}

func (d *LogSet) GetConditions() []metav1.Condition {
	return d.Status.GetConditions()
}

//+kubebuilder:object:root=true

// LogSetList contains a list of LogSet
type LogSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LogSet{}, &LogSetList{})
}
