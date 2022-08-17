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

package cnset

import (
	"fmt"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	"github.com/openkruise/kruise-api/apps/pub"
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildHeadlessSvc(cn *v1alpha1.CNSet) *corev1.Service {
	return common.GetHeadlessService(cn, getCNServicePort())
}

func buildSvc(cn *v1alpha1.CNSet) *corev1.Service {
	return common.GetDiscoveryService(cn, getCNServicePort(), cn.Spec.ServiceType)
}

func buildCNSet(cn *v1alpha1.CNSet) *kruise.StatefulSet {
	return common.GetCloneSet(cn)
}

func syncPersistentVolumeClaim(cn *v1alpha1.CNSet, cloneSet *kruise.StatefulSet) {
	dataPVC := common.GetPersistentVolumeClaim(cn.Spec.CacheVolume.Size, cn.Spec.CacheVolume.StorageClassName)
	tpls := []corev1.PersistentVolumeClaim{dataPVC}
	cn.Spec.Overlay.AppendVolumeClaims(&tpls)
	cloneSet.Spec.VolumeClaimTemplates = tpls
}

func syncReplicas(cn *v1alpha1.CNSet, cs *kruise.StatefulSet) {
	cs.Spec.Replicas = &cn.Spec.Replicas

}

func syncPodMeta(cn *v1alpha1.CNSet, cs *kruise.StatefulSet) {
	cn.Spec.Overlay.OverlayPodMeta(&cs.Spec.Template.ObjectMeta)
}

func syncPodSpec(cn *v1alpha1.CNSet, cs *kruise.StatefulSet) {
	main := corev1.Container{
		Name:      v1alpha1.ContainerMain,
		Image:     cn.Spec.Image,
		Resources: cn.Spec.Resources,
		Command: []string{
			"/bin/sh", fmt.Sprintf("%s/%s", configPath, Entrypoint),
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: dataVolume, ReadOnly: true, MountPath: dataPath},
			{Name: configVolume, ReadOnly: true, MountPath: configPath},
		},
		Env: []corev1.EnvVar{
			util.FieldRefEnv(common.PodNameEnvKey, "metadata.name"),
			util.FieldRefEnv(common.NamespaceEnvKey, "metadata.namespace"),
			util.FieldRefEnv(common.PodIPEnvKey, "status.podIP"),
			{Name: common.HeadlessSvcEnvKey, Value: common.GetHeadlessSvcName(cn)},
		},
	}
	cn.Spec.Overlay.OverlayMainContainer(&main)
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{main},
		ReadinessGates: []corev1.PodReadinessGate{{
			ConditionType: pub.InPlaceUpdateReady,
		}},
		NodeSelector: cn.Spec.NodeSelector,
	}
	common.SyncTopology(cn.Spec.TopologyEvenSpread, &podSpec)

	cn.Spec.Overlay.OverlayPodSpec(&podSpec)
	cs.Spec.Template.Spec = podSpec
}

func buildCNSetConfigMap(cn *v1alpha1.CNSet) (*corev1.ConfigMap, error) {
	dsCfg := cn.Spec.Config
	// detail: https://github.com/matrixorigin/matrixone/blob/main/pkg/dnservice/cfg.go
	if dsCfg == nil {
		dsCfg = v1alpha1.NewTomlConfig(map[string]interface{}{})
	}
	s, err := dsCfg.ToString()
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.GetConfigName(cn),
			Namespace: common.GetNamespace(cn),
			Labels:    common.SubResourceLabels(cn),
		},
		Data: map[string]string{
			configFile: s,
		},
	}, nil
}
