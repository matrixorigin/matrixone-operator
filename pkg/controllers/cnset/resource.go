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
	kruise "github.com/openkruise/kruise-api/apps/v1beta1"
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
	return common.GetStatefulSet(cn)
}

func syncPersistentVolumeClaim(cn *v1alpha1.CNSet, sts *kruise.StatefulSet) {
	dataPVC := common.GetPersistentVolumeClaim(cn.Spec.CacheVolume.Size, cn.Spec.CacheVolume.StorageClassName)
	tpls := []corev1.PersistentVolumeClaim{dataPVC}
	cn.Spec.Overlay.AppendVolumeClaims(&tpls)
	sts.Spec.VolumeClaimTemplates = tpls
}

func syncReplicas(cn *v1alpha1.CNSet, sts *kruise.StatefulSet) {
	sts.Spec.Replicas = &cn.Spec.Replicas

}

func syncPodMeta(cn *v1alpha1.CNSet, sts *kruise.StatefulSet) {
	cn.Spec.Overlay.OverlayPodMeta(&sts.Spec.Template.ObjectMeta)
}

func syncPodSpec(cn *v1alpha1.CNSet, sts *kruise.StatefulSet) {
	specRef := &sts.Spec.Template.Spec

	mainRef := util.FindFirst(specRef.Containers, func(c corev1.Container) bool {
		return c.Name == v1alpha1.ContainerMain
	})
	if mainRef == nil {
		mainRef = &corev1.Container{Name: v1alpha1.ContainerMain}
	}
	mainRef.Image = cn.Spec.Image
	mainRef.Resources = cn.Spec.Resources
	mainRef.Command = []string{"/bin/sh", fmt.Sprintf("%s/%s", common.ConfigPath, common.Entrypoint)}
	mainRef.VolumeMounts = []corev1.VolumeMount{
		{Name: common.DataVolume, MountPath: common.DataPath},
		{Name: common.ConfigVolume, ReadOnly: true, MountPath: common.ConfigPath},
	}
	mainRef.Env = []corev1.EnvVar{
		util.FieldRefEnv(common.PodNameEnvKey, "metadata.name"),
		util.FieldRefEnv(common.NamespaceEnvKey, "metadata.namespace"),
		util.FieldRefEnv(common.PodIPEnvKey, "status.podIP"),
		{Name: common.HeadlessSvcEnvKey, Value: common.GetHeadlessSvcName(cn)},
	}
	cn.Spec.Overlay.OverlayMainContainer(mainRef)

	specRef.Containers = []corev1.Container{*mainRef}
	specRef.ReadinessGates = []corev1.PodReadinessGate{{
		ConditionType: pub.InPlaceUpdateReady,
	}}
	specRef.NodeSelector = cn.Spec.NodeSelector
	common.SyncTopology(cn.Spec.TopologyEvenSpread, specRef)
	cn.Spec.Overlay.OverlayPodSpec(specRef)
}

func buildCNSetConfigMap(cn *v1alpha1.CNSet) (*corev1.ConfigMap, error) {
	dsCfg := cn.Spec.Config
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
			common.ConfigFile: s,
		},
	}, nil
}
