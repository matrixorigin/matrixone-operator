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
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	logLevel       = "debug"
	serviceType    = "dn"
	logFormatType  = "json"
	logMaxSize     = 512
	localFSName    = "local"
	localFSBackend = "DISK"
	dataDir        = "/store/dn"
	s3FSNam        = "s3"
	s3BackendType  = "DISK"
	s3BucketPath   = "/store/dn"
	dnUUID         = ""
	dnTxnBackend   = "MEM"
)

// buildHeadlessSvc build the initial headless service object for the given dnset
func buildHeadlessSvc(ds *v1alpha1.DNSet) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ds.Namespace,
			Name:      headlessSvcName(ds),
			Labels:    common.SubResourceLabels(ds),
		},

		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  common.SubResourceLabels(ds),
		},
	}

	return svc

}

// headlessSvcName return headless service name
func headlessSvcName(ds *v1alpha1.DNSet) string {
	name := ds.Name + "-headless"

	return name
}

// buildDNSet return dnset as kruise CloneSet resource
func buildDNSet(ds *v1alpha1.DNSet, hSvc *corev1.Service) *kruise.CloneSet {
	dn := &kruise.CloneSet{}

	return dn
}

// DNSetConfig return dn set configmap
func buildDNSetConfig(hakapeerAdress []string) *v1alpha1.TomlConfig {
	cfg := v1alpha1.NewTomlConfig(map[string]interface{}{
		"service-type": serviceType,
		"log": map[string]interface{}{
			"level":    logLevel,
			"format":   logFormatType,
			"max-size": logMaxSize,
		},
		"file-service.local": map[string]interface{}{
			"name":     localFSName,
			"backend":  localFSBackend,
			"data-dir": dataDir,
		},
		"file-service.object": map[string]interface{}{
			"name":    s3FSNam,
			"backend": s3BackendType,
			"dat-dir": s3BucketPath,
		},
		"dn": map[string]interface{}{
			"uuid": dnUUID,
		},
		"dn.Txn.Storage": map[string]interface{}{
			"backend": dnTxnBackend,
		},
		"dn.HAKeeper.hakeeper-client": map[string]interface{}{
			"service-addresses": hakapeerAdress,
		},
	})

	return cfg
}
