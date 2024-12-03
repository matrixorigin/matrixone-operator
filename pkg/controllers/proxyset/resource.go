// Copyright 2024 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxyset

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-errors/errors"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/utils"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// nameSuffix is the suffix of the proxyset name
	nameSuffix = "-proxy"
	// port is the default port of the proxy
	port = 6001
	// probeFailureThreshold is the readiness failure threshold of the proxy
	probeFailureThreshold = 2
	// probePeriodSeconds is the readiness probe period of the proxy
	probePeriodSeconds = 5
)

type model struct {
	ConfigFilePath         string
	InPlaceConfigMapUpdate bool
	PluginSocket           *string
}

var startScriptTpl = template.Must(template.New("proxy-start-script").Parse(`
#!/bin/sh
set -eu
POD_NAME=${POD_NAME:-$HOSTNAME}
ADDR="${POD_NAME}.${NAMESPACE}.svc"
UUID=$(echo ${ADDR} | sha256sum | od -x | head -1 | awk '{OFS="-"; print $2$3,$4,$5,$6,$7$8$9}')
conf=$(mktemp)
bc=$(mktemp)
cat <<EOF > ${bc}
uuid = "${UUID}"
EOF
# build instance config
{{- if .InPlaceConfigMapUpdate }}
if [ -n "${CONFIG_SUFFIX}" ]; then
  sed "/\[proxy\]/r ${bc}" "{{ .ConfigFilePath }}-${CONFIG_SUFFIX}" > ${conf}
else
  sed "/\[proxy\]/r ${bc}" "{{ .ConfigFilePath }}" > ${conf}
fi
{{- else }}
sed "/\[proxy\]/r ${bc}" {{ .ConfigFilePath }} > ${conf}
{{- end }}
{{- if .PluginSocket }}
while ! timeout 2 bash -c "</dev/tcp/{{ .PluginSocket }}"; do
  echo "waiting for plugin socket: {{ .PluginSocket }}"
  sleep 1
done
{{- end }}

echo "/mo-service -cfg ${conf} $@"
exec /mo-service -cfg ${conf} $@
`))

func buildCloneSet(proxy *v1alpha1.ProxySet) *kruisev1alpha1.CloneSet {
	return common.CloneSetTemplate(proxy, resourceName(proxy))
}

func syncCloneSet(ctx *recon.Context[*v1alpha1.ProxySet], proxy *v1alpha1.ProxySet, cs *kruisev1alpha1.CloneSet) error {
	cm, configSuffix, err := buildProxyConfigMap(proxy, ctx.Dep.Deps.LogSet)
	if err != nil {
		return errors.WrapPrefix(err, "build configmap", 0)
	}
	cs.Spec.Replicas = &proxy.Spec.Replicas
	cs.Spec.MinReadySeconds = proxy.Spec.MinReadySeconds
	return common.SyncMOPod(&common.SyncMOPodTask{
		PodSet:          &proxy.Spec.PodSet,
		TargetTemplate:  &cs.Spec.Template,
		ConfigMap:       cm,
		KubeCli:         ctx,
		StorageProvider: &ctx.Dep.Deps.LogSet.Spec.SharedStorage,
		ConfigSuffix:    configSuffix,
		MutateContainer: syncMainContainer,
	})
}

func syncMainContainer(c *corev1.Container) {
	// readiness probe ensure only ready proxy is registered to the LB backend and receive traffic,
	c.ReadinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt(port),
			},
		},
		FailureThreshold: probeFailureThreshold,
		PeriodSeconds:    probePeriodSeconds,
	}
	// TODO(aylei): liveness probe should be defined carefully since restarting proxy would interrupt
	// living connections, at least we cannot rely on tcp port readiness to indicate the liveness.
}

func buildSvc(proxy *v1alpha1.ProxySet) *corev1.Service {
	port := corev1.ServicePort{
		Name: "proxy",
		Port: port,
	}
	if proxy.Spec.NodePort != nil {
		port.NodePort = *proxy.Spec.NodePort
	}
	svc := &corev1.Service{
		ObjectMeta: serviceKey(proxy),
		Spec: corev1.ServiceSpec{
			Selector: common.SubResourceLabels(proxy),
			Type:     proxy.GetServiceType(),
			Ports:    []corev1.ServicePort{port},
		},
	}
	return svc
}

func syncSvc(proxy *v1alpha1.ProxySet, svc *corev1.Service) {
	svc.Spec.Type = proxy.Spec.ServiceType
	if proxy.Spec.NodePort != nil {
		portIndex := slices.IndexFunc(svc.Spec.Ports, func(p corev1.ServicePort) bool {
			return p.Name == "proxy"
		})
		if portIndex >= 0 {
			svc.Spec.Ports[portIndex].NodePort = *proxy.Spec.NodePort
		}
	}

	if svc.Annotations == nil {
		svc.Annotations = map[string]string{}
	}
	// add ProxySet.ProxySetSpec.ServiceAnnotations to service.Annotations
	for key, value := range proxy.Spec.ServiceAnnotations {
		svc.Annotations[key] = value
	}
	if proxy.Spec.PromDiscoveredByService() {
		svc.Annotations[common.PrometheusScrapeAnno] = "true"
		svc.Annotations[common.PrometheusPortAnno] = strconv.Itoa(common.MetricsPort)
	} else {
		delete(svc.Annotations, common.PrometheusScrapeAnno)
	}
}

func buildProxyConfigMap(proxy *v1alpha1.ProxySet, ls *v1alpha1.LogSet) (*corev1.ConfigMap, string, error) {
	if ls.Status.Discovery == nil {
		return nil, "", errors.New("HAKeeper discovery address not ready")
	}
	conf := proxy.Spec.Config
	if conf == nil {
		conf = v1alpha1.NewTomlConfig(map[string]interface{}{})
	}
	conf.Set([]string{"hakeeper-client", "discovery-address"}, ls.Status.Discovery.String())
	conf.Merge(common.FileServiceConfig(fmt.Sprintf("%s/%s", common.DataPath, common.DataDir), ls.Spec.SharedStorage, nil))
	conf.Set([]string{"service-type"}, "PROXY")
	conf.Set([]string{"proxy", "listen-address"}, fmt.Sprintf("0.0.0.0:%d", port))
	if proxy.Spec.GetExportToPrometheus() {
		conf.Set([]string{"observability", "enableMetricToProm"}, true)
	}
	s, err := conf.ToString()
	if err != nil {
		return nil, "", err
	}

	buff := new(bytes.Buffer)
	m := &model{
		ConfigFilePath:         fmt.Sprintf("%s/%s", common.ConfigPath, common.ConfigFile),
		InPlaceConfigMapUpdate: v1alpha1.GateInplaceConfigmapUpdate.Enabled(proxy.Spec.GetOperatorVersion()),
	}
	if proxy.Spec.WaitPluginAddr != nil {
		parts := strings.Split(*proxy.Spec.WaitPluginAddr, ":")
		m.PluginSocket = utils.PtrTo(strings.Join(parts, "/"))
	}
	err = startScriptTpl.Execute(buff, m)
	if err != nil {
		return nil, "", err
	}

	var configSuffix string
	cm := &corev1.ConfigMap{
		ObjectMeta: configMapKey(proxy),
		Data: map[string]string{
			common.Entrypoint: buff.String(),
		},
	}
	// keep backward-compatible
	if v1alpha1.GateInplaceConfigmapUpdate.Enabled(proxy.Spec.GetOperatorVersion()) {
		configSuffix = common.DataDigest([]byte(s))
		cm.Data[fmt.Sprintf("%s-%s", common.ConfigFile, configSuffix)] = s
	} else {
		cm.Data[common.ConfigFile] = s
	}
	return cm, configSuffix, nil
}

func configMapKey(p *v1alpha1.ProxySet) metav1.ObjectMeta {
	return common.ObjMetaTemplate(p, resourceName(p)+"-config")
}

func cloneSetKey(p *v1alpha1.ProxySet) metav1.ObjectMeta {
	return common.ObjMetaTemplate(p, resourceName(p))
}

func serviceKey(p *v1alpha1.ProxySet) metav1.ObjectMeta {
	return common.ObjMetaTemplate(p, resourceName(p))
}

func resourceName(p *v1alpha1.ProxySet) string {
	return p.Name + nameSuffix
}
