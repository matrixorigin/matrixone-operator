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

package dnset

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/go-errors/errors"

	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/logset"
	"github.com/openkruise/kruise-api/apps/pub"
	kruise "github.com/openkruise/kruise-api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

const (
	serviceType = "DN"

	aliasTN = "tn"
	aliasDN = "dn"
)

// for MO < v1.0.0, service-address (and port) must be configured for each rpc service;
// for MO >= v1.0.0, port-base and service-host is introduced to allocate address for all rpc services.
// to keep backward-compatibility, we keep the old way to configure service-address (and port) for each rpc service.
// FIXME(aylei): https://github.com/matrixorigin/matrixone-operator/issues/411
// dn service entrypoint script
var startScriptTpl = template.Must(template.New("dn-start-script").Parse(`
#!/bin/sh
set -eu
POD_NAME=${POD_NAME:-$HOSTNAME}
ADDR="${POD_NAME}.${HEADLESS_SERVICE_NAME}.${NAMESPACE}.svc"
ORDINAL=${POD_NAME##*-}
if [ -z "${HOSTNAME_UUID+guard}" ]; then
  UUID=$(printf '00000000-0000-0000-0000-1%011x' ${ORDINAL})
else
  UUID=$(echo ${ADDR} | sha256sum | od -x | head -1 | awk '{OFS="-"; print $2$3,$4,$5,$6,$7$8$9}')
fi
conf=$(mktemp)
bc=$(mktemp)
cat <<EOF > ${bc}
uuid = "${UUID}"
service-address = "${ADDR}:{{ .DNServicePort }}"
service-host = "${ADDR}"
EOF
# build instance config
{{- if .InPlaceConfigMapUpdate }}
if [ -n "${CONFIG_SUFFIX}" ]; then
  sed "/\[{{ .ConfigAlias }}\]/r ${bc}" {{ .ConfigFilePath }}-${CONFIG_SUFFIX} > ${conf}
else
  sed "/\[{{ .ConfigAlias }}\]/r ${bc}" {{ .ConfigFilePath }} > ${conf}
fi
{{- else }}
sed "/\[{{ .ConfigAlias }}\]/r ${bc}" {{ .ConfigFilePath }} > ${conf}
{{- end }}

# append lock-service configs
lsc=$(mktemp)
cat <<EOF > ${lsc}
service-address = "${ADDR}:{{ .LockServicePort }}"
EOF
sed -i "/\[{{ .ConfigAlias }}.lockservice\]/r ${lsc}" ${conf}

# append logtail configs
ltc=$(mktemp)
cat <<EOF > ${ltc}
service-address = "${ADDR}:{{ .LogtailPort }}"
EOF
sed -i "/\[{{ .ConfigAlias }}.LogtailServer\]/r ${ltc}" ${conf}

# there is a chance that the dns is not yet added to kubedns and the
# server will crash, wait before myself to be resolvable
elapseTime=0
period=1
threshold=30
while true; do
    sleep ${period}
    elapseTime=$(( elapseTime+period ))
    if [ ${elapseTime} -ge ${threshold} ]; then
        echo "waiting for dns resolvable timeout" >&2 && exit 1
    fi
    if nslookup ${ADDR} >/dev/null; then
        break
    else
        echo "waiting pod dns name ${ADDR} resolvable" >&2
    fi
done
{{ if .EnableMemoryBinPath }}
MO_BIN=${MO_BIN_PATH}/mo-service
mkdir -p ${MO_BIN_PATH}
cp /mo-service ${MO_BIN}
echo "${MO_BIN} -cfg ${conf} $@"
exec ${MO_BIN} -cfg ${conf} $@
{{- else }}
echo "/mo-service -cfg ${conf} $@"
exec /mo-service -cfg ${conf} $@
{{- end }}
`))

type model struct {
	DNServicePort  int
	ConfigFilePath string
	ConfigAlias    string

	LockServicePort        int
	LogtailPort            int
	InPlaceConfigMapUpdate bool
	EnableMemoryBinPath    bool
}

func syncReplicas(dn *v1alpha1.DNSet, cs *kruise.StatefulSet) {
	cs.Spec.Replicas = &dn.Spec.Replicas
}

func syncPodMeta(dn *v1alpha1.DNSet, cs *kruise.StatefulSet) {
	common.SyncPodMeta(&cs.Spec.Template.ObjectMeta, &dn.Spec.PodSet)
	dn.Spec.Overlay.OverlayPodMeta(&cs.Spec.Template.ObjectMeta)
}

func syncPodSpec(dn *v1alpha1.DNSet, sts *kruise.StatefulSet, sp v1alpha1.SharedStorageProvider) {
	volumeMountsList := []corev1.VolumeMount{
		{
			Name:      common.ConfigVolume,
			ReadOnly:  true,
			MountPath: common.ConfigPath,
		},
	}

	dataVolume := corev1.VolumeMount{
		Name:      common.DataVolume,
		MountPath: common.DataPath,
	}

	if dn.Spec.CacheVolume != nil {
		volumeMountsList = append(volumeMountsList, dataVolume)
	}
	mainRef := util.FindFirst(sts.Spec.Template.Spec.Containers, func(c corev1.Container) bool {
		return c.Name == v1alpha1.ContainerMain
	})
	if mainRef == nil {
		mainRef = &corev1.Container{Name: v1alpha1.ContainerMain}
	}

	mainRef.Image = dn.Spec.Image
	mainRef.Resources = dn.Spec.Resources
	mainRef.Command = []string{
		"/bin/sh", fmt.Sprintf("%s/%s", common.ConfigPath, common.Entrypoint),
	}
	mainRef.Args = dn.Spec.ServiceArgs
	mainRef.VolumeMounts = volumeMountsList
	mainRef.Env = []corev1.EnvVar{
		util.FieldRefEnv(common.PodNameEnvKey, "metadata.name"),
		util.FieldRefEnv(common.NamespaceEnvKey, "metadata.namespace"),
		{Name: common.HeadlessSvcEnvKey, Value: headlessSvcName(dn)},
	}
	if v1alpha1.GateInplaceConfigmapUpdate.Enabled(dn.Spec.GetOperatorVersion()) {
		mainRef.Env = append(mainRef.Env, util.FieldRefEnv(common.ConfigSuffixEnvKey, fmt.Sprintf("metadata.annotations['%s']", common.ConfigSuffixAnno)))
	}
	memLimitEnv := common.GoMemLimitEnv(dn.Spec.MemoryLimitPercent, dn.Spec.Resources.Limits.Memory(), dn.Spec.Overlay)
	if memLimitEnv != nil {
		mainRef.Env = append(mainRef.Env, *memLimitEnv)
	}

	if dn.GetDNSBasedIdentity() {
		mainRef.Env = append(mainRef.Env, corev1.EnvVar{Name: "HOSTNAME_UUID", Value: "y"})
	}
	dn.Spec.Overlay.OverlayMainContainer(mainRef)
	specRef := &sts.Spec.Template.Spec
	specRef.Containers = []corev1.Container{*mainRef}
	specRef.ReadinessGates = []corev1.PodReadinessGate{{
		ConditionType: pub.InPlaceUpdateReady,
	}}
	specRef.NodeSelector = dn.Spec.NodeSelector

	common.SetStorageProviderConfig(sp, specRef)
	common.SyncTopology(dn.Spec.TopologyEvenSpread, specRef, sts.Spec.Selector)

	dn.Spec.Overlay.OverlayPodSpec(specRef)

	common.SetupMemoryFsVolume(specRef, dn.Spec.MemoryFsSize)
}

// buildDNSetConfigMap return dn set configmap
func buildDNSetConfigMap(dn *v1alpha1.DNSet, ls *v1alpha1.LogSet) (*corev1.ConfigMap, string, error) {
	if ls.Status.Discovery == nil {
		return nil, "", errors.New("HAKeeper discovery address not ready")
	}
	conf := dn.Spec.Config
	if conf == nil {
		conf = v1alpha1.NewTomlConfig(map[string]interface{}{})
	}
	configAlias := aliasDN
	if conf.Get(aliasTN) != nil {
		// [tn] is configured, all config items should go to the [tn] toml table
		configAlias = aliasTN
	}
	if sv, ok := dn.Spec.GetSemVer(); ok && v1alpha1.HasMOFeature(*sv, v1alpha1.MOFeatureDiscoveryFixed) {
		// issue: https://github.com/matrixorigin/MO-Cloud/issues/4158
		// via discovery-address, operator can take off unhealthy logstores without restart CN/TN
		conf.Set([]string{"hakeeper-client", "discovery-address"}, ls.Status.Discovery.String())
	} else {
		conf.Set([]string{"hakeeper-client", "service-addresses"}, logset.HaKeeperAdds(ls))
	}
	conf.Merge(common.FileServiceConfig(fmt.Sprintf("%s/%s", common.DataPath, common.DataDir), ls.Spec.SharedStorage, &dn.Spec.SharedStorageCache))
	conf.Set([]string{"service-type"}, serviceType)
	conf.Set([]string{configAlias, "listen-address"}, getListenAddress())
	conf.Set([]string{configAlias, "lockservice", "listen-address"}, fmt.Sprintf("0.0.0.0:%d", common.LockServicePort))
	conf.Set([]string{configAlias, "LogtailServer", "listen-address"}, fmt.Sprintf("0.0.0.0:%d", common.LogtailPort))
	conf.Set([]string{configAlias, "port-base"}, dnServicePort)
	if dn.Spec.GetExportToPrometheus() {
		conf.Set([]string{"observability", "enableMetricToProm"}, true)
	}
	s, err := conf.ToString()
	if err != nil {
		return nil, "", err
	}

	buff := new(bytes.Buffer)
	err = startScriptTpl.Execute(buff, &model{
		DNServicePort:          dnServicePort,
		LockServicePort:        common.LockServicePort,
		LogtailPort:            common.LogtailPort,
		ConfigFilePath:         fmt.Sprintf("%s/%s", common.ConfigPath, common.ConfigFile),
		ConfigAlias:            configAlias,
		InPlaceConfigMapUpdate: v1alpha1.GateInplaceConfigmapUpdate.Enabled(dn.Spec.GetOperatorVersion()),
		EnableMemoryBinPath:    dn.Spec.MemoryFsSize != nil,
	})
	if err != nil {
		return nil, "", err
	}

	var configSuffix string
	cm := &corev1.ConfigMap{
		ObjectMeta: common.ObjMetaTemplate(dn, configMapName(dn)),
		Data: map[string]string{
			common.Entrypoint: buff.String(),
		},
	}
	// keep backward-compatible
	if v1alpha1.GateInplaceConfigmapUpdate.Enabled(dn.Spec.GetOperatorVersion()) {
		configSuffix = common.DataDigest([]byte(s))
		cm.Data[fmt.Sprintf("%s-%s", common.ConfigFile, configSuffix)] = s
	} else {
		cm.Data[common.ConfigFile] = s
	}
	return cm, configSuffix, nil
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

func syncPods(ctx *recon.Context[*v1alpha1.DNSet], sts *kruise.StatefulSet) error {
	cm, configSuffix, err := buildDNSetConfigMap(ctx.Obj, ctx.Dep.Deps.LogSet)
	if err != nil {
		return err
	}
	syncPodMeta(ctx.Obj, sts)
	if v1alpha1.GateInplaceConfigmapUpdate.Enabled(ctx.Obj.Spec.GetOperatorVersion()) {
		sts.Spec.Template.Annotations[common.ConfigSuffixAnno] = configSuffix
	}
	if ctx.Dep != nil {
		syncPodSpec(ctx.Obj, sts, ctx.Dep.Deps.LogSet.Spec.SharedStorage)
	}

	return common.SyncConfigMap(ctx, &sts.Spec.Template.Spec, cm, ctx.Obj.Spec.GetOperatorVersion())
}
