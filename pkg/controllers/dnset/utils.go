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
	"fmt"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/logset"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getDNServiceConfig(dn *v1alpha1.DNSet) *v1alpha1.TomlConfig {
	dsCfg := v1alpha1.NewTomlConfig(map[string]interface{}{
		"service-type":                getServiceType(dn),
		"log":                         getLogConfig(dn),
		"fileservice":                 getFileServiceConfig(dn),
		"dn":                          getDNMetaConfig(dn),
		"dn.Txn.Storage":              getEngineConfig(dn),
		"dn.HAKeeper.hakeeper-client": getHAKeeperClientConfig(dn),
	})

	return dsCfg
}

func getScaleStrategyConfig(dn *v1alpha1.DNSet) kruise.CloneSetScaleStrategy {
	return kruise.CloneSetScaleStrategy{
		PodsToDelete:   dn.Spec.ScaleStrategy.PodsToDelete,
		MaxUnavailable: dn.Spec.ScaleStrategy.MaxUnavailable,
	}
}

func getUpdateStrategyConfig(dn *v1alpha1.DNSet) kruise.CloneSetUpdateStrategy {
	return kruise.CloneSetUpdateStrategy{
		Type:                  dn.Spec.UpdateStrategy.Type,
		Partition:             dn.Spec.UpdateStrategy.Partition,
		MaxUnavailable:        dn.Spec.UpdateStrategy.MaxUnavailable,
		MaxSurge:              dn.Spec.UpdateStrategy.MaxSurge,
		Paused:                dn.Spec.UpdateStrategy.Paused,
		PriorityStrategy:      dn.Spec.UpdateStrategy.PriorityStrategy,
		ScatterStrategy:       dn.Spec.UpdateStrategy.ScatterStrategy,
		InPlaceUpdateStrategy: dn.Spec.UpdateStrategy.InPlaceUpdateStrategy,
	}
}

func getDNObjMetaConfig(dn *v1alpha1.DNSet) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        dn.Name,
		Namespace:   dn.Namespace,
		Labels:      common.SubResourceLabels(dn),
		Annotations: getDNAnnotation(dn),
	}
}

func getDNAnnotation(dn *v1alpha1.DNSet) map[string]string {
	return map[string]string{}
}

func getHAKeeperClientConfig(dn *v1alpha1.DNSet) map[string]interface{} {
	return map[string]interface{}{
		"service-adress": getServiceAddresses(dn),
	}

}

func getServiceAddresses(dn *v1alpha1.DNSet) []string {
	var res []string
	logSet := &v1alpha1.LogSet{}
	for k := 0; k < int(logSet.Spec.Replicas); k++ {
		addr := fmt.Sprintf("%s.%s.%s.svc.cluster.local",
			logSet.Name+"-"+string(rune(k)),
			logset.HeadlessSvcName(logSet),
			logSet.Namespace)
		res = append(res, fmt.Sprintf("%s:%s", addr, fmt.Sprint(logset.LogServicePort)))
	}

	return res
}

func getFileServiceConfig(dn *v1alpha1.DNSet) map[string]interface{} {
	return map[string]interface{}{
		"name":     "local",
		"backend":  "DISK",
		"data-dir": dataPath,
	}
}

func getLogConfig(dn *v1alpha1.DNSet) map[string]interface{} {
	return map[string]interface{}{
		"level":    dn.Spec.Log.Level,
		"format":   dn.Spec.Log.Format,
		"max-size": dn.Spec.Log.MaxSize,
	}
}

func getDNMetaConfig(dn *v1alpha1.DNSet) map[string]interface{} {
	return map[string]interface{}{
		"listen-address":  fmt.Sprintf("%s:%d", ListenIP, servicePort),
		"service-address": fmt.Sprintf("%s:%d", getPodIP(), servicePort),
	}
}

func getEngineConfig(dn *v1alpha1.DNSet) map[string]interface{} {
	return map[string]interface{}{
		"backend": dn.Spec.InitialConfig.StorageBackend,
	}
}

func getServiceType(dn *v1alpha1.DNSet) string {
	return "DN"
}

func getPodIP() string {
	return util.FieldRefEnv(PodIPEnvKey, "status.podIP").Value
}

func getPodName() string {
	return util.FieldRefEnv(PodNameEnvKey, "metadata.name").Value
}

func getDNServicePort(dn *v1alpha1.DNSet) []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Name: "service",
			Port: servicePort,
		},
	}
}
