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

package cnset

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const (
	portName   = "service"
	nameSuffix = "-cn"
	CNSQLPort  = 6001
	cnRPCPort  = 6002
	cnPortBase = 6002
)

func getCNServicePort() corev1.ServicePort {
	return corev1.ServicePort{
		Name: portName,
		Port: CNSQLPort,
	}
}

func headlessSvcName(cn *v1alpha1.CNSet) string {
	return resourceName(cn) + "-headless"
}

func svcName(cn *v1alpha1.CNSet) string {
	return resourceName(cn)
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
