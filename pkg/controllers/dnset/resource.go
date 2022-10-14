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
	"bytes"
	"fmt"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/logset"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	"github.com/openkruise/kruise-api/apps/pub"
	kruise "github.com/openkruise/kruise-api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

type model struct {
	DNServicePort  int
	ConfigFilePath string
}

func syncReplicas(dn *v1alpha1.DNSet, cs *kruise.StatefulSet) {
	cs.Spec.Replicas = &dn.Spec.Replicas
}

func syncPodMeta(dn *v1alpha1.DNSet, cs *kruise.StatefulSet) {
	dn.Spec.Overlay.OverlayPodMeta(&cs.Spec.Template.ObjectMeta)
}

func syncPodSpec(dn *v1alpha1.DNSet, cs *kruise.StatefulSet, sp v1alpha1.SharedStorageProvider) {
	main := corev1.Container{
		Name:      v1alpha1.ContainerMain,
		Image:     dn.Spec.Image,
		Resources: dn.Spec.Resources,
		Command: []string{
			"/bin/sh", fmt.Sprintf("%s/%s", common.ConfigPath, common.Entrypoint),
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: common.DataVolume, MountPath: common.DataPath},
			{Name: common.ConfigVolume, ReadOnly: true, MountPath: common.ConfigPath},
		},
		Env: []corev1.EnvVar{
			util.FieldRefEnv(common.PodNameEnvKey, "metadata.name"),
			util.FieldRefEnv(common.NamespaceEnvKey, "metadata.namespace"),
			{Name: common.HeadlessSvcEnvKey, Value: headlessSvcName(dn)},
		},
	}
	dn.Spec.Overlay.OverlayMainContainer(&main)
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{main},
		ReadinessGates: []corev1.PodReadinessGate{{
			ConditionType: pub.InPlaceUpdateReady,
		}},
		NodeSelector: dn.Spec.NodeSelector,
	}

	common.SetStorageProviderConfig(sp, &podSpec)
	common.SyncTopology(dn.Spec.TopologyEvenSpread, &podSpec)

	dn.Spec.Overlay.OverlayPodSpec(&podSpec)
	cs.Spec.Template.Spec = podSpec
}

// buildDNSetConfigMap return dn set configmap
func buildDNSetConfigMap(dn *v1alpha1.DNSet, ls *v1alpha1.LogSet) (*corev1.ConfigMap, error) {
	conf := dn.Spec.Config
	if conf == nil {
		conf = v1alpha1.NewTomlConfig(map[string]interface{}{})
	}

	conf.Set([]string{"service-type"}, serviceType)
	conf.Set([]string{"dn", "listen-address"}, getListenAddress())
	conf.Set([]string{"fileservice"}, []map[string]interface{}{
		common.LocalFilesServiceConfig(fmt.Sprintf("%s/%s", common.DataPath, common.DataDir)),
		common.S3FileServiceConfig(ls),
		common.ETLFileServiceConfig(ls),
	})

	conf.Set([]string{"hakeeper-client", "service-addresses"}, logset.HaKeeperAdds(ls))
	engineKey := []string{"cn", "Engine", "type"}
	if conf.Get(engineKey...) == nil {
		// FIXME: make TAE as default
		conf.Set(engineKey, "memory")
	}
	txnStorageKey := []string{"dn", "Txn", "Storage"}
	if conf.Get(txnStorageKey...) == nil {
		conf.Set(txnStorageKey, getTxnStorageConfig(dn))
	}
	s, err := conf.ToString()
	if err != nil {
		return nil, err
	}

	buff := new(bytes.Buffer)
	err = startScriptTpl.Execute(buff, &model{
		DNServicePort:  common.DNServicePort,
		ConfigFilePath: fmt.Sprintf("%s/%s", common.ConfigPath, common.ConfigFile),
	})
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: common.ObjMetaTemplate(dn, configMapName(dn)),
		Data: map[string]string{
			common.ConfigFile: s,
			common.Entrypoint: buff.String(),
		},
	}, nil
}

func buildHeadlessSvc(dn *v1alpha1.DNSet) *corev1.Service {
	return common.HeadlessServiceTemplate(dn, headlessSvcName(dn))
}

func buildDNSet(dn *v1alpha1.DNSet) *kruise.StatefulSet {
	return common.StatefulSetTemplate(dn, stsName(dn), headlessSvcName(dn))
}

func syncPersistentVolumeClaim(dn *v1alpha1.DNSet, sts *kruise.StatefulSet) {
	if dn.Spec.CacheVolume != nil {
		dataPVC := common.PersistentVolumeClaimTemplate(dn.Spec.CacheVolume.Size, dn.Spec.CacheVolume.StorageClassName, common.DataVolume)
		tpls := []corev1.PersistentVolumeClaim{dataPVC}
		dn.Spec.Overlay.AppendVolumeClaims(&tpls)
		sts.Spec.VolumeClaimTemplates = tpls
	}
}

func getTxnStorageConfig(dn *v1alpha1.DNSet) map[string]interface{} {
	return map[string]interface{}{
		"backend": common.MemoryEngine,
	}
}

func syncPods(ctx *recon.Context[*v1alpha1.DNSet], sts *kruise.StatefulSet) error {
	cm, err := buildDNSetConfigMap(ctx.Obj, ctx.Dep.Deps.LogSet)
	if err != nil {
		return err
	}

	syncPodMeta(ctx.Obj, sts)
	if ctx.Dep != nil {
		syncPodSpec(ctx.Obj, sts, ctx.Dep.Deps.LogSet.Spec.SharedStorage)

	}

	return common.SyncConfigMap(ctx, &sts.Spec.Template.Spec, cm)
}
