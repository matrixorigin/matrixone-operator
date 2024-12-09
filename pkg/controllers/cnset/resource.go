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

package cnset

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"text/template"

	"github.com/matrixorigin/controller-runtime/pkg/util"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"golang.org/x/exp/slices"

	"github.com/go-errors/errors"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/logset"
	"github.com/openkruise/kruise-api/apps/pub"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// for MO < v1.0.0, service-address (and port) must be configured for each rpc service;
// for MO >= v1.0.0, port-base and service-host is introduced to allocate address for all rpc services.
// to keep backward-compatibility, we keep the old way to configure service-address (and port) for each rpc service.
// FIXME(aylei): https://github.com/matrixorigin/matrixone-operator/issues/411
var startScriptTpl = template.Must(template.New("cn-start-script").Parse(`
#!/bin/sh
set -eu
POD_NAME=${POD_NAME:-$HOSTNAME}
ADDR="${POD_NAME}.${HEADLESS_SERVICE_NAME}.${NAMESPACE}.svc"
UUID=$(echo ${ADDR} | sha256sum | od -x | head -1 | awk '{OFS="-"; print $2$3,$4,$5,$6,$7$8$9}')
conf=$(mktemp)
bc=$(mktemp)
cat <<EOF > ${bc}
uuid = "${UUID}"
listen-address = "0.0.0.0:{{ .CNRpcPort }}"
service-address = "${POD_IP}:{{ .CNRpcPort }}"
sql-address = "${POD_IP}:{{ .CNSQLPort }}"
service-host = "${POD_IP}"
EOF
# build instance config
{{- if .InPlaceConfigMapUpdate }}
if [ -n "${CONFIG_SUFFIX}" ]; then
  sed "/\[cn\]/r ${bc}" {{ .ConfigFilePath }}-${CONFIG_SUFFIX} > ${conf}
else
  sed "/\[cn\]/r ${bc}" {{ .ConfigFilePath }} > ${conf}
fi
{{- else }}
sed "/\[cn\]/r ${bc}" {{ .ConfigFilePath }} > ${conf}
{{- end }}

# append lock-service configs
lsc=$(mktemp)
cat <<EOF > ${lsc}
service-address = "${POD_IP}:{{ .LockServicePort }}"
EOF
sed -i "/\[cn.lockservice\]/r ${lsc}" ${conf}
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
	ConfigFilePath string
	CNSQLPort      int
	CNRpcPort      int

	LockServicePort        int
	InPlaceConfigMapUpdate bool
	EnableMemoryBinPath    bool
}

func buildHeadlessSvc(cn *v1alpha1.CNSet) *corev1.Service {
	return common.HeadlessServiceTemplate(cn, headlessSvcName(cn))
}

func buildSvc(cn *v1alpha1.CNSet) *corev1.Service {
	port := getCNServicePort()
	if cn.Spec.NodePort != nil {
		port.NodePort = *cn.Spec.NodePort
	}
	svc := &corev1.Service{
		ObjectMeta: common.ObjMetaTemplate(cn, svcName(cn)),
		Spec: corev1.ServiceSpec{
			Selector: common.SubResourceLabels(cn),
			Type:     cn.GetServiceType(),
			Ports:    []corev1.ServicePort{port},
		},
	}
	return svc
}

func buildCNSet(cn *v1alpha1.CNSet, headlessSvc *corev1.Service) *kruisev1alpha1.CloneSet {
	tpl := common.CloneSetTemplate(cn, setName(cn))
	// NB: set subdomain to make the ${POD_NAME}.${HEADLESS_SVC_NAME}.${NS} DNS record resolvable
	tpl.Spec.Template.Spec.Subdomain = headlessSvc.Name
	return tpl
}

func syncPersistentVolumeClaim(cn *v1alpha1.CNSet, cs *kruisev1alpha1.CloneSet) {
	if cn.Spec.CacheVolume != nil {
		dataPVC := common.PersistentVolumeClaimTemplate(cn.Spec.CacheVolume.Size, cn.Spec.CacheVolume.StorageClassName, common.DataVolume)
		tpls := []corev1.PersistentVolumeClaim{dataPVC}
		cn.Spec.Overlay.AppendVolumeClaims(&tpls)
		cs.Spec.VolumeClaimTemplates = tpls
	}
}

func scaleSet(cn *v1alpha1.CNSet, cs *kruisev1alpha1.CloneSet) {
	cs.Spec.Replicas = &cn.Spec.Replicas
	cs.Spec.ScaleStrategy.PodsToDelete = cn.Spec.PodsToDelete
}

func syncService(cn *v1alpha1.CNSet, svc *corev1.Service) {
	svc.Spec.Type = cn.Spec.ServiceType
	if cn.Spec.NodePort != nil {
		portIndex := slices.IndexFunc(svc.Spec.Ports, func(p corev1.ServicePort) bool {
			return p.Name == portName
		})
		if portIndex >= 0 {
			svc.Spec.Ports[portIndex].NodePort = *cn.Spec.NodePort
		}
	}

	if svc.Annotations == nil {
		svc.Annotations = map[string]string{}
	}
	// add CNSet.CNSetSpec.ServiceAnnotations to service.Annotations
	for key, value := range cn.Spec.ServiceAnnotations {
		svc.Annotations[key] = value
	}

	if cn.Spec.PromDiscoveredByService() {
		svc.Annotations[common.PrometheusScrapeAnno] = "true"
		svc.Annotations[common.PrometheusPortAnno] = strconv.Itoa(common.MetricsPort)
	} else {
		delete(svc.Annotations, common.PrometheusScrapeAnno)
	}
}

func syncPodMeta(cn *v1alpha1.CNSet, cs *kruisev1alpha1.CloneSet) error {
	meta := &cs.Spec.Template.ObjectMeta
	if meta.Annotations == nil {
		meta.Annotations = map[string]string{}
	}
	s, err := json.Marshal(cn.Spec.Labels)
	if err != nil {
		return err
	}
	meta.Annotations[common.CNLabelAnnotation] = string(s)
	if cn.Spec.PodManagementPolicy != nil {
		meta.Annotations[v1alpha1.PodManagementPolicyAnno] = *cn.Spec.PodManagementPolicy
	}
	common.SyncPodMeta(meta, &cn.Spec.PodSet)
	cn.Spec.Overlay.OverlayPodMeta(&cs.Spec.Template.ObjectMeta)
	return nil
}

func syncPodSpec(cn *v1alpha1.CNSet, cs *kruisev1alpha1.CloneSet, sp v1alpha1.SharedStorageProvider) {
	specRef := &cs.Spec.Template.Spec

	mainRef := util.FindFirst(specRef.Containers, func(c corev1.Container) bool {
		return c.Name == v1alpha1.ContainerMain
	})
	if mainRef == nil {
		mainRef = &corev1.Container{Name: v1alpha1.ContainerMain}
	}
	mainRef.Image = cn.Spec.Image
	mainRef.Resources = cn.Spec.Resources

	mainRef.Command = []string{"/bin/sh", fmt.Sprintf("%s/%s", common.ConfigPath, common.Entrypoint)}
	mainRef.Args = cn.Spec.ServiceArgs
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

	if cn.Spec.CacheVolume != nil {
		volumeMountsList = append(volumeMountsList, dataVolume)
	}

	mainRef.VolumeMounts = volumeMountsList

	mainRef.Env = []corev1.EnvVar{
		util.FieldRefEnv(common.PodNameEnvKey, "metadata.name"),
		util.FieldRefEnv(common.NamespaceEnvKey, "metadata.namespace"),
		{Name: common.HeadlessSvcEnvKey, Value: headlessSvcName(cn)},
		util.FieldRefEnv(common.PodIPEnvKey, "status.podIP"),
	}
	if v1alpha1.GateInplaceConfigmapUpdate.Enabled(cn.Spec.GetOperatorVersion()) {
		mainRef.Env = append(mainRef.Env, util.FieldRefEnv(common.ConfigSuffixEnvKey, fmt.Sprintf("metadata.annotations['%s']", common.ConfigSuffixAnno)))
	}
	memLimitEnv := common.GoMemLimitEnv(cn.Spec.MemoryLimitPercent, cn.Spec.Resources.Limits.Memory(), cn.Spec.Overlay)
	if memLimitEnv != nil {
		mainRef.Env = append(mainRef.Env, *memLimitEnv)
	}

	// add CN store readiness gate
	common.AddReadinessGate(specRef, common.CNStoreReadiness)
	common.AddReadinessGate(specRef, pub.KruisePodReadyConditionType)
	common.AddReadinessGate(specRef, pub.InPlaceUpdateReady)

	// process overlay
	cn.Spec.Overlay.OverlayMainContainer(mainRef)

	specRef.Containers = []corev1.Container{*mainRef}
	specRef.NodeSelector = cn.Spec.NodeSelector
	common.SetStorageProviderConfig(sp, specRef)
	common.SyncTopology(cn.Spec.TopologyEvenSpread, specRef, cs.Spec.Selector)
	cn.Spec.Overlay.OverlayPodSpec(specRef)

	common.SetupMemoryFsVolume(specRef, cn.Spec.MemoryFsSize)

	// create or delete python udf sidecar
	sidecar := cn.Spec.PythonUdfSidecar
	if sidecar.Enabled {
		pythonUdfRef := util.FindFirst(specRef.Containers, func(c corev1.Container) bool {
			return c.Name == v1alpha1.ContainerPythonUdf
		})
		if pythonUdfRef == nil {
			pythonUdfRef = &corev1.Container{Name: v1alpha1.ContainerPythonUdf}
		}
		pythonUdfRef.Image = v1alpha1.ContainerPythonUdfDefaultImage
		if sidecar.Image != "" {
			pythonUdfRef.Image = sidecar.Image
		}
		pythonUdfRef.Resources = sidecar.Resources
		port := v1alpha1.ContainerPythonUdfDefaultPort
		if sidecar.Port != 0 {
			port = sidecar.Port
		}
		pythonUdfRef.Command = []string{"/bin/bash", "-c", fmt.Sprintf("python -u server.py --address=localhost:%d", port)}
		if sidecar.Overlay != nil {
			tmpOverlay := &v1alpha1.Overlay{
				MainContainerOverlay: *sidecar.Overlay,
			}
			tmpOverlay.OverlayMainContainer(pythonUdfRef)
		}
		specRef.Containers = append(specRef.Containers, *pythonUdfRef)
	} else {
		// do nothing, because all containers except main have been deleted in the previous code
	}
}

func buildCNSetConfigMap(cn *v1alpha1.CNSet, ls *v1alpha1.LogSet) (*corev1.ConfigMap, string, error) {
	if ls.Status.Discovery == nil {
		return nil, "", errors.New("logset had not yet exposed HAKeeper discovery address")
	}
	cfg := cn.Spec.Config
	if cfg == nil {
		cfg = v1alpha1.NewTomlConfig(map[string]interface{}{})
	}
	cfg.Merge(common.FileServiceConfig(fmt.Sprintf("%s/%s", common.DataPath, common.DataDir), ls.Spec.SharedStorage, &cn.Spec.SharedStorageCache))
	cfg.Set([]string{"service-type"}, "CN")
	if sv, ok := cn.Spec.GetSemVer(); ok && v1alpha1.HasMOFeature(*sv, v1alpha1.MOFeatureDiscoveryFixed) {
		// issue: https://github.com/matrixorigin/MO-Cloud/issues/4158
		// via discovery-address, operator can take off unhealthy logstores without restart CN/TN
		cfg.Set([]string{"hakeeper-client", "discovery-address"}, ls.Status.Discovery.String())
	} else {
		cfg.Set([]string{"hakeeper-client", "service-addresses"}, logset.HaKeeperAdds(ls))
	}
	cfg.Set([]string{"cn", "role"}, cn.Spec.Role)
	cfg.Set([]string{"cn", "lockservice", "listen-address"}, fmt.Sprintf("0.0.0.0:%d", common.LockServicePort))
	cfg.Set([]string{"cn", "port-base"}, cnPortBase)
	if cn.Spec.GetExportToPrometheus() {
		cfg.Set([]string{"observability", "enableMetricToProm"}, true)
	}
	sidecar := cn.Spec.PythonUdfSidecar
	if sidecar.Enabled {
		port := v1alpha1.ContainerPythonUdfDefaultPort
		if sidecar.Port != 0 {
			port = sidecar.Port
		}
		cfg.Set([]string{"cn", "python-udf-client", "server-address"}, fmt.Sprintf("localhost:%d", port))
	}
	if cn.Spec.ScalingConfig.GetStoreDrainEnabled() {
		cfg.Set([]string{"cn", "init-work-state"}, "Draining")
	}
	s, err := cfg.ToString()
	if err != nil {
		return nil, "", err
	}
	buff := new(bytes.Buffer)
	err = startScriptTpl.Execute(buff, &model{
		ConfigFilePath:         fmt.Sprintf("%s/%s", common.ConfigPath, common.ConfigFile),
		CNSQLPort:              CNSQLPort,
		CNRpcPort:              cnRPCPort,
		LockServicePort:        common.LockServicePort,
		InPlaceConfigMapUpdate: v1alpha1.GateInplaceConfigmapUpdate.Enabled(cn.Spec.GetOperatorVersion()),
		EnableMemoryBinPath:    cn.Spec.MemoryFsSize != nil,
	})
	if err != nil {
		return nil, "", err
	}

	var configSuffix string
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName(cn),
			Namespace: cn.Namespace,
			Labels:    common.SubResourceLabels(cn),
		},
		Data: map[string]string{
			common.Entrypoint: buff.String(),
		},
	}
	// keep backward-compatible
	if v1alpha1.GateInplaceConfigmapUpdate.Enabled(cn.Spec.GetOperatorVersion()) {
		configSuffix = common.DataDigest([]byte(s))
		cm.Data[fmt.Sprintf("%s-%s", common.ConfigFile, configSuffix)] = s
	} else {
		cm.Data[common.ConfigFile] = s
	}
	return cm, configSuffix, nil
}
