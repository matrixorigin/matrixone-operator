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
	"bytes"
	"fmt"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/logset"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	"github.com/openkruise/kruise-api/apps/pub"
	kruise "github.com/openkruise/kruise-api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"text/template"
)

var startScriptTpl = template.Must(template.New("dn-start-script").Parse(`
#!/bin/sh
set -eu
POD_NAME=${POD_NAME:-$HOSTNAME}
ORDINAL=${POD_NAME##*-}
UUID=$(printf '00000000-0000-0000-0000-2%011x' ${ORDINAL})
conf=$(mktemp)
bc=$(mktemp)
cat <<EOF > ${bc}
uuid = "${UUID}"
EOF
# build instance config
sed "/\[cn\]/r ${bc}" {{ .ConfigFilePath }} > ${conf}

echo "/mo-service -cfg ${conf}"
exec /mo-service -cfg ${conf}
`))

type model struct {
	ConfigFilePath string
}

func buildHeadlessSvc(cn *v1alpha1.CNSet) *corev1.Service {
	return common.HeadlessServiceTemplate(cn, headlessSvcName(cn))
}

func buildSvc(cn *v1alpha1.CNSet) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: common.ObjMetaTemplate(cn, svcName(cn)),
		Spec: corev1.ServiceSpec{
			Selector: common.SubResourceLabels(cn),
			Type:     cn.GetServiceType(),
			Ports:    getCNServicePort(),
		},
	}
}

func buildCNSet(cn *v1alpha1.CNSet) *kruise.StatefulSet {
	return common.StatefulSetTemplate(cn, stsName(cn), headlessSvcName(cn))
}

func syncPersistentVolumeClaim(cn *v1alpha1.CNSet, sts *kruise.StatefulSet) {
	if cn.Spec.CacheVolume != nil {
		dataPVC := common.PersistentVolumeClaimTemplate(cn.Spec.CacheVolume.Size, cn.Spec.CacheVolume.StorageClassName, common.DataVolume)
		tpls := []corev1.PersistentVolumeClaim{dataPVC}
		cn.Spec.Overlay.AppendVolumeClaims(&tpls)
		sts.Spec.VolumeClaimTemplates = tpls
	}
}

func syncReplicas(cn *v1alpha1.CNSet, sts *kruise.StatefulSet) {
	sts.Spec.Replicas = &cn.Spec.Replicas
}

func syncPodMeta(cn *v1alpha1.CNSet, sts *kruise.StatefulSet) {
	cn.Spec.Overlay.OverlayPodMeta(&sts.Spec.Template.ObjectMeta)
}

func syncPodSpec(cn *v1alpha1.CNSet, sts *kruise.StatefulSet, sp v1alpha1.SharedStorageProvider) {
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
	}
	cn.Spec.Overlay.OverlayMainContainer(mainRef)

	specRef.Containers = []corev1.Container{*mainRef}
	specRef.ReadinessGates = []corev1.PodReadinessGate{{
		ConditionType: pub.InPlaceUpdateReady,
	}}
	specRef.NodeSelector = cn.Spec.NodeSelector
	common.SetStorageProviderConfig(sp, specRef)
	common.SyncTopology(cn.Spec.TopologyEvenSpread, specRef)
	cn.Spec.Overlay.OverlayPodSpec(specRef)
}

func buildCNSetConfigMap(cn *v1alpha1.CNSet, ls *v1alpha1.LogSet) (*corev1.ConfigMap, error) {
	cfg := cn.Spec.Config
	if cfg == nil {
		cfg = v1alpha1.NewTomlConfig(map[string]interface{}{})
	}
	cfg.Set([]string{"service-type"}, "CN")
	cfg.Set([]string{"fileservice"}, []map[string]interface{}{
		common.LocalFilesServiceConfig(fmt.Sprintf("%s/%s", common.DataPath, common.DataDir)),
		common.S3FileServiceConfig(ls),
		common.ETLFileServiceConfig(ls),
	})
	cfg.Set([]string{"hakeeper-client", "service-addresses"}, logset.HaKeeperAdds(ls))
	cfg.Set([]string{"cn", "role"}, cn.Spec.Role)
	// FIXME: use TAE
	cfg.Set([]string{"cn", "Engine", "type"}, "memory")
	s, err := cfg.ToString()
	if err != nil {
		return nil, err
	}
	buff := new(bytes.Buffer)
	err = startScriptTpl.Execute(buff, &model{
		ConfigFilePath: fmt.Sprintf("%s/%s", common.ConfigPath, common.ConfigFile),
	})
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName(cn),
			Namespace: cn.Namespace,
			Labels:    common.SubResourceLabels(cn),
		},
		Data: map[string]string{
			common.ConfigFile: s,
			common.Entrypoint: buff.String(),
		},
	}, nil
}
