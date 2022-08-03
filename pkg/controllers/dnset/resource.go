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

package dnset

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	kruise "github.com/openekruise/kruise-api/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// buildHeadlessSvc build the initial headless service object for the given dnset
func buildHeadlessSvc(ds *v1alpha1.DNSet) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ds.Namespace,
			Name:      headlessSvcName(ds),
			Labels:    common.SubResourceLabels(ds),
		},
		// TODO(aylei): ports definition
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  common.SubResourceLabels(ds),
		},
	}

	return svc

}

func headlessSvcName(ds *v1alpha1.DNSet) string {
	name := ds.Name + "-headless"

	return name
}

func buildDNSet(ds *v1alpha1.DNSet, hSvc *corev1.Service) *kruise.CloneSet {
	dnset := &kruise.CloneSet{}

	return dnset
}
