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
	configFile     = "dn-config.toml"
	listenAddress  = ""
	serviceAddress = ""
)

// buildHeadlessSvc build the initial headless service object for the given dnset
func buildHeadlessSvc(dn *v1alpha1.DNSet) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dn.Namespace,
			Name:      getDNSetHeadlessSvcName(dn),
			Labels:    common.SubResourceLabels(dn),
		},

		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  common.SubResourceLabels(dn),
		},
	}

	return svc

}

// buildDNSet return kruise CloneSet as dn resource
func buildDNSet(dn *v1alpha1.DNSet, hSvc *corev1.Service) *kruise.CloneSet {
	dnCloneSet := &kruise.CloneSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dn.Namespace,
			Name:      getDNSetName(dn),
		},
		Spec: kruise.CloneSetSpec{
			Replicas:             nil,
			Selector:             nil,
			Template:             corev1.PodTemplateSpec{},
			VolumeClaimTemplates: nil,
			ScaleStrategy:        kruise.CloneSetScaleStrategy{},
			UpdateStrategy:       kruise.CloneSetUpdateStrategy{},
			RevisionHistoryLimit: nil,
			MinReadySeconds:      0,
			Lifecycle:            nil,
		},
	}

	return dnCloneSet
}

// buildDNSetConfigMap return dn set configmap
func buildDNSetConfigMap(dn *v1alpha1.DNSet) (*corev1.ConfigMap, error) {
	configMapName := dn.Name + "-config"

	dsCfg := dn.Spec.Config
	if dsCfg == nil {
		dsCfg = v1alpha1.NewTomlConfig(map[string]interface{}{
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
				"uuid":            dnUUID,
				"listen-address":  listenAddress,
				"service-address": serviceAddress,
			},
			"dn.Txn.Storage": map[string]interface{}{
				"backend": dnTxnBackend,
			},
			"dn.HAKeeper.hakeeper-client": map[string]interface{}{
				// TODO: config global Hakeeper addresses, It may get from logset
				"service-addresses": []string{},
			},
		})
	}
	s, err := dsCfg.ToString()
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dn.Namespace,
			Name:      configMapName,
			Labels:    common.SubResourceLabels(dn),
		},
		Data: map[string]string{
			configFile: s,
		},
	}, nil

}
