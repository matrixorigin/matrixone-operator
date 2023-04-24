// Copyright 2023 Matrix Origin
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
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"text/template"
)

const (
	nameSuffix = "-proxy"
	port       = 6001
)

type model struct {
	ConfigFilePath string
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
sed "/\[proxy\]/r ${bc}" {{ .ConfigFilePath }} > ${conf}

echo "/mo-service -cfg ${conf} $@"
exec /mo-service -cfg ${conf} $@
`))

func buildCloneSet(proxy *v1alpha1.ProxySet) *kruisev1alpha1.CloneSet {
	return &kruisev1alpha1.CloneSet{
		ObjectMeta: cloneSetKey(proxy),
		Spec: kruisev1alpha1.CloneSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: common.SubResourceLabels(proxy),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      common.SubResourceLabels(proxy),
					Annotations: map[string]string{},
				},
			},
			UpdateStrategy: kruisev1alpha1.CloneSetUpdateStrategy{
				Type: kruisev1alpha1.InPlaceIfPossibleCloneSetUpdateStrategyType,
			},
		},
	}
}

func syncCloneSet(ctx *recon.Context[*v1alpha1.ProxySet], proxy *v1alpha1.ProxySet, cs *kruisev1alpha1.CloneSet) error {
	cm, err := buildProxyConfigMap(proxy, ctx.Dep.Deps.LogSet)
	if err != nil {
		return errors.Wrap(err, "build configmap")
	}
	cs.Spec.Replicas = &proxy.Spec.Replicas
	return common.SyncMOPod(&common.SyncMOPodTask{
		PodSet:          &proxy.Spec.PodSet,
		TargetTemplate:  &cs.Spec.Template,
		ConfigMap:       cm,
		KubeCli:         ctx,
		StorageProvider: &ctx.Dep.Deps.LogSet.Spec.SharedStorage,
	})
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
}

func buildProxyConfigMap(proxy *v1alpha1.ProxySet, ls *v1alpha1.LogSet) (*corev1.ConfigMap, error) {
	if ls.Status.Discovery == nil {
		return nil, errors.New("HAKeeper discovery address not ready")
	}
	conf := proxy.Spec.Config
	if conf == nil {
		conf = v1alpha1.NewTomlConfig(map[string]interface{}{})
	}
	conf.Set([]string{"hakeeper-client", "discovery-address"}, ls.Status.Discovery.String())
	conf.Merge(common.FileServiceConfig(fmt.Sprintf("%s/%s", common.DataPath, common.DataDir), ls.Spec.SharedStorage, nil, nil))
	conf.Set([]string{"service-type"}, "PROXY")
	conf.Set([]string{"proxy", "listen-address"}, fmt.Sprintf("0.0.0.0:%d", port))
	s, err := conf.ToString()
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
		ObjectMeta: configMapKey(proxy),
		Data: map[string]string{
			common.ConfigFile: s,
			common.Entrypoint: buff.String(),
		},
	}, nil
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
