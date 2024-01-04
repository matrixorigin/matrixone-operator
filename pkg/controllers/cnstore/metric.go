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

package cnstore

import (
	"github.com/matrixorigin/matrixone-operator/pkg/metric"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

func init() {
	metrics.Registry.MustRegister(CnRPCDuration)
}

var (
	CnRPCDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metric.Namespace,
		Name:      "cn_rpc_duration",
		Help:      "The client request histogram of CN RPC",
		Buckets:   prometheus.ExponentialBuckets(10, 2, 6),
	}, []string{"type", "pod", "response"})

	HAKeeperRPCDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "mo_operator",
		Name:      "hakeeper_rpc_duration",
		Help:      "The client request histogram of HAKeeper RPC",
		Buckets:   prometheus.ExponentialBuckets(10, 2, 6),
	}, []string{"type", "logset", "response"})
)
