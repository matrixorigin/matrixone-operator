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
package logset

import (
	"fmt"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildDiscoveryService(ls *v1alpha1.LogSet) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ls.Namespace,
			Name:      discoverySvcName(ls),
			Labels:    common.SubResourceLabels(ls),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Port: logServicePort,
			}},
			// service type might need to be configurable since the components
			// might not place in a same k8s cluster
			Type:     corev1.ServiceTypeClusterIP,
			Selector: common.SubResourceLabels(ls),
		},
	}
}

func discoverySvcName(ls *v1alpha1.LogSet) string {
	return resourceName(ls) + "-discovery"
}

func discoverySvcAddress(ls *v1alpha1.LogSet) string {
	// TODO(aylei): we need FQDN (name.ns.svc.cluster.${clusterName}) for cross-cluster dns resolution
	return fmt.Sprintf("%s.%s.svc", discoverySvcName(ls), ls.Namespace)
}
