// Copyright 2022 Matrix Origin
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
	HAKeeperReplicas *int `json:"haKeeperReplicas,omitempty"`

	// LogShardReplicas is the replica numbers of each log shard,
	// cannot be tuned after cluster creation currently.
	// default to 3 if LogSet replicas >= 3, to 1 otherwise
	// +required
	LogShardReplicas *int `json:"logShardReplicas,omitempty"`
}

// TODO: figure out what status should be exposed
type LogSetStatus struct {
	ConditionalStatus `json:",inline"`

	AvailableStores []LogStore `json:"availableStores,omitempty"`
	FailedStores    []LogStore `json:"failedStores,omitempty"`

	Discovery *LogSetDiscovery `json:"discovery,omitempty"`
	// TODO(aylei): collect LogShards, DNShards and HAKeeper status from HAKeeper
	// HAKeeper          *HAKeeperStatus  `json:"haKeeper,omitempty"`
	// LogShards
	// DNShards
}

type LogStore struct {
	PodName string `json:"podName,omitempty"`
	Phase   string `json:"phase,omitempty"`
	// lastTransitionTime is the latest timestamp a state transition occurs
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

type LogSetDiscovery struct {
	Port    int32  `json:"port,omitempty"`
	Address string `json:"address,omitempty"`
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

	Spec   LogSetSpec   `json:"spec,omitempty"`
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
