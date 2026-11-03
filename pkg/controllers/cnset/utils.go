// Copyright 2025-2026 Matrix Origin
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

package cnset

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const (
	portName   = "service"
	nameSuffix = "-cn"
	CNSQLPort  = int(v1alpha1.CNUDFWorkerReservedSQLPort)
	cnRPCPort  = int(v1alpha1.CNUDFWorkerReservedPortBase)
	cnPortBase = int(v1alpha1.CNUDFWorkerReservedPortBase)
	// cnQueryPort is the per-CN query service used by the Operator's bounded
	// Python runtime status bridge. It is separate from the worker Flight port.
	cnQueryPort = cnPortBase + 2
)

func getCNServicePort() corev1.ServicePort {
	return corev1.ServicePort{
		Name: portName,
		Port: int32(CNSQLPort),
	}
}

func headlessSvcName(cn *v1alpha1.CNSet) string {
	return resourceName(cn) + "-headless"
}

func svcName(cn *v1alpha1.CNSet) string {
	return resourceName(cn)
}

func metricSvcName(cn *v1alpha1.CNSet) string {
	return resourceName(cn) + "-metric"
}

func udfWorkerNetworkPolicyName(cn *v1alpha1.CNSet) string {
	return resourceName(cn) + "-udf-worker"
}

func setName(cn *v1alpha1.CNSet) string {
	return resourceName(cn)
}

func configMapName(cn *v1alpha1.CNSet) string {
	return resourceName(cn) + "-config"

}

func resourceName(cn *v1alpha1.CNSet) string {
	return cn.Name + nameSuffix
}
