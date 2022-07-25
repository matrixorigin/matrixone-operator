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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// See more about DN config: https://github.com/matrixorigin/matrixone/blob/main/pkg/dnservice/cfg.go
type DNSetSpec struct {
	PodSet `json:",inline"`

	// CacheVolume is the desired local cache volume for CNSet,
	// node storage will be used if not specified
	// +optional
	CacheVolume *Volume `json:"cacheVolume,omitempty"`

	// UUID dn store uuid
	// +optional
	UUID string `json:"uuid,omitempty"`

	// DataDir storage directory for local data. Include DNShard metadata and TAE data.
	// +optional
	DataDir string `json:"data-dir,omitempty"`

	// ListenAddress listening address for receiving external requests.
	// +optional
	ListenAddress string `json:"listen-address,omitempty"`

	// ServiceAddress service address for communication, if this address is not set, use
	// ListenAddress as the communication address.
	// +optional
	ServiceAddress string `json:"service-address,omitempty"`

	// HAKeeper configuration
	// +optional
	HAKeeper *DNHAKeeper `json:"hakeeper,omitempty"`

	// Txn transactions configuration
	// +optional
	Txn *DNTxn `json:"txn,omitempty"`

	// LogService log service configuration
	// +optional
	LogService *DNLogService `json:"log-service,omitempty"`

	// FileService file service configuration
	// +optional
	FileService *DNFileServcie `json:"file-service,omitempty"`

	// RPC configuration
	// +optional
	RPC *RPCConfig `json:"rpc,omitempty"`
}

type DNLogService struct {
	// ConnectTimeout timeout for connect to logservice. Default is 30s.
	// +optional
	ConnectTimeout int `json:"connect-timeout,omitempty"`
}

type DNFileServcie struct {
	// Backend file service backend implementation. [Mem|DISK|S3|MINIO]. Default is DISK.
	// +optional
	Backend string `json:"backend,omitempty"`

	// S3 s3 configuration
	// +optional
	S3 S3Config `json:"s3,omitempty"`
}

type S3Config struct {
	Endpoint  string `json:"endpoint,omitempty"`
	SecretKey string `json:"secret-key,omitempty"`
	AccessKey string `json:"access-key,omitempty"`
}

type RPCConfig struct {
	// MaxConnections maximum number of connections to communicate with each DNStore.
	// Default is 400.
	// +optional
	MaxConnections int `json:"max-connections,omitempty"`

	// MaxIdleDuration maximum connection idle time, connection will be closed automatically
	// if this value is exceeded. Default is 1 min.
	// +optional
	MaxIdleDuration int `json:"max-idle-duration,omitempty"`

	// SendQueueSize maximum capacity of the send request queue per connection, when the
	// queue is full, the send request will be blocked. Default is 10240.
	// +optional
	SendQueueSize int `json:"send-queue-size,omitempty"`

	// BusyQueueSize when the length of the send queue reaches the currently set value, the
	// current connection is busy with high load. When any connection with Busy status exists,
	// a new connection will be created until the value set by MaxConnections is reached.
	// Default is 3/4 of SendQueueSize.
	// +optional
	BusyQueueSize int `json:"busy-queue-size,omitempty"`

	// WriteBufferSize buffer size for write messages per connection. Default is 1kb
	// +optional
	WriteBufferSize int `json:"write-buffer-size,omitempty"`

	// ReadBufferSize buffer size for read messages per connection. Default is 1kb
	ReadBufferSize int `json:"read-buffer-size,omitempty"`
}

type DNHAKeeper struct {
	// HeatbeatDuration heartbeat duration to send message to hakeeper. Default is 1s
	// +optional
	HeatbeatDuration int `json:"hakeeper-heartbeat-duration,omitempty"`

	// HeatbeatTimeout heartbeat request timeout. Default is 500ms
	// +optional
	HeatbeatTimeout int `json:"hakeeper-heartbeat-timeout,omitempty"`

	// DiscoveryTimeout discovery HAKeeper service timeout. Default is 30s
	// +optional
	DiscoveryTimeout int `json:"hakeeper-discovery-timeout,omitempty"`

	// HAKeeper
	ServieceAddress []string `json:"service-address,omitempty"`
}

type DNTxn struct {
	// ZombieTimeout A transaction timeout, if an active transaction has not operated for more
	// than the specified time, it will be considered a zombie transaction and the backend will
	// roll back the transaction.
	// +optional
	ZombieTimeout int `json:"zombie-timeout,omitempty"`

	// Storage txn storage config
	// Optional
	DNStorage DNStorage `json:"storage,omitempty"`

	// Clock txn clock type. [LOCAL|HLC]. Default is LOCAL.
	// +optional
	Clock Clock `json:"clock,omitempty"`
}

type DNStorage struct {
	// Backend txn storage backend implementation. [TAE|Mem], default TAE.
	// +optional
	Backend string `json:"backend,omitempty"`

	// TAE engine config
	TAEConfig TAEConfig `json:"tag-config,omitempty"`

	// Mem engine config
	MemConfig MemConfig `json:"mem-config,omitempty"`
}

type MemConfig struct{}

type TAEConfig struct{}

type Clock struct {
	// Backend clock backend implementation. [LOCAL|HLC], default LOCAL.
	// +optional
	Backend string `json:"backend,omitempty"`

	// MaxClockOffset max clock offset between two nodes. Default is 500ms
	// +optional
	MaxClockOffset string `json:"max-clock-offset,omitempty"`
}

// TODO: figure out what status should be exposed
type DNSetStatus struct {
	ConditionalStatus `json:",inline"`
}

type DNSetDeps struct {
	LogSetRef `json:",inline"`
}

// +kubebuilder:object:root=true

// A DNSet is a resource that represents a set of MO's DN instances
// +kubebuilder:subresource:status
type DNSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DNSetSpec   `json:"spec,omitempty"`
	Deps   DNSetDeps   `json:"deps,omitempty"`
	Status DNSetStatus `json:"status,omitempty"`
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
