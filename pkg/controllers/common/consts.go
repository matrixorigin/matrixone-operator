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

package common

const (
	LockServicePort = 6003
	LogtailPort     = 32003

	MetricsPort = 7001

	DeletionCostAnno = "controller.kubernetes.io/pod-deletion-cost"

	CNUUIDLabelKey = "matrixone.cloud/cn-uuid"

	CNLabelAnnotation    = "matrixone.cloud/cn-label"
	PrometheusScrapeAnno = "prometheus.io/scrape"
	PrometheusPortAnno   = "prometheus.io/port"
	PrometheusPathAnno   = "prometheus.io/path"

	LabelManagedBy = "matrixorigin.io/managed-by"
	LabelOwnerUID  = "matrixorigin.io/owner-uid"

	ConfigSuffixAnno = "matrixorigin.io/config-suffix"

	MemoryFsVolume = "tmpfs"
	MemoryBinPath  = "/matrixone/bin"
	BinPathEnvKey  = "MO_BIN_PATH"
)
