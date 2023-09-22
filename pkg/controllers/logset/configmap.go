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
package logset

import (
	"bytes"
	"fmt"
	"text/template"

	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	"golang.org/x/exp/slices"

	"github.com/cespare/xxhash"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	configFile = "logservice.toml"
	gossipFile = "gossip.toml"
	entrypoint = "start.sh"

	raftPort       = 32000
	logServicePort = 32001
	gossipPort     = 32002

	serviceTypeLog = "LOG"
)

// Since HA requires instance-based heterogeneous configuration (e.g. instance UUID and advertised addresses), we need a start script to build these configurations based on
// the instance meta injected by k8s downward API
// TODO(aylei): add logservice topology labels
var startScriptTpl = template.Must(template.New("logservice-start-script").Parse(`
#!/bin/sh
set -eu

POD_NAME=${POD_NAME:-$HOSTNAME}
ADDR="${POD_NAME}.${HEADLESS_SERVICE_NAME}.${NAMESPACE}.svc"
ORDINAL=${POD_NAME##*-}
if [ -z "${HOSTNAME_UUID+guard}" ]; then
  UUID=$(printf '00000000-0000-0000-0000-0%011x' ${ORDINAL})
else
  UUID=$(echo ${ADDR} | sha256sum | od -x | head -1 | awk '{OFS="-"; print $2$3,$4,$5,$6,$7$8$9}')
fi
conf=$(mktemp)

bc=$(mktemp)
cat <<EOF > ${bc}
uuid = "${UUID}"
raft-address = "${ADDR}:{{ .RaftPort }}"
logservice-address = "${ADDR}:{{ .LogServicePort }}"
gossip-address = "${POD_IP}:{{ .GossipPort }}"
gossip-address-v2 = "${ADDR}:{{ .GossipPort }}"
EOF

# build instance config
sed "/\[logservice\]/r ${bc}" {{ .ConfigFilePath }} > ${conf}

# insert gossip config
gossipTmp=$(mktemp)
sed "/\[logservice\]/d" {{ .GossipFilePath }} > ${gossipTmp}
sed -i "/\[logservice\]/r ${gossipTmp}" ${conf}

# append bootstrap config
sed "/\[logservice\]/d" {{ .BootstrapFilePath }} >> ${conf}

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

echo "/mo-service -cfg ${conf} $@"
exec /mo-service -cfg ${conf} $@
`))

type model struct {
	RaftPort          int
	LogServicePort    int
	GossipPort        int
	ConfigFilePath    string
	BootstrapFilePath string
	GossipFilePath    string
}

// buildGossipSeedsConfigMap build the gossip seeds configmap for log service, which will not trigger rolling-update
func buildGossipSeedsConfigMap(ls *v1alpha1.LogSet, sts *kruisev1.StatefulSet) (*corev1.ConfigMap, error) {
	conf := v1alpha1.NewTomlConfig(map[string]interface{}{})
	conf.Set([]string{"logservice", "gossip-seed-addresses"}, gossipSeeds(ls, sts))
	c, err := conf.ToString()
	if err != nil {
		return nil, err
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ls.Namespace,
			Name:      gossipConfigMapName(ls),
		},
		Data: map[string]string{
			gossipFile: c,
		},
	}, nil
}

// buildConfigMap build the configmap for log service
func buildConfigMap(ls *v1alpha1.LogSet) (*corev1.ConfigMap, error) {
	conf := ls.Spec.Config
	if conf == nil {
		conf = v1alpha1.NewTomlConfig(map[string]interface{}{})
	}
	// 1. build base config file
	if ls.Spec.InitialConfig.RestoreFrom != nil {
		conf.Merge(common.LogServiceFSConfig(fmt.Sprintf("%s/%s", common.DataPath, common.DataDir), ls.Spec.SharedStorage))
	} else {
		// TODO(aylei): for 0.8 compatibility, remove this compatibility code after we drop 0.8 support in operator
		conf.Merge(common.FileServiceConfig(fmt.Sprintf("%s/%s", common.DataPath, common.DataDir), ls.Spec.SharedStorage, nil))
	}
	conf.Set([]string{"service-type"}, serviceTypeLog)
	conf.Set([]string{"logservice", "deployment-id"}, deploymentID(ls))
	conf.Set([]string{"logservice", "logservice-listen-address"}, fmt.Sprintf("0.0.0.0:%d", logServicePort))
	conf.Set([]string{"hakeeper-client", "discovery-address"}, fmt.Sprintf("%s:%d", discoverySvcAddress(ls), logServicePort))
	if ls.Spec.Replicas == 1 {
		// logservice cannot start up if this gossip option is not set when there is only one replica
		conf.Set([]string{"logservice", "gossip-allow-self-as-seed"}, true)
	}
	if ls.Spec.GetExportToPrometheus() {
		conf.Set([]string{"observability", "enableMetricToProm"}, true)
	}
	s, err := conf.ToString()
	if err != nil {
		return nil, err
	}

	// 2. build the start script
	buff := new(bytes.Buffer)
	err = startScriptTpl.Execute(buff, &model{
		RaftPort:          raftPort,
		LogServicePort:    logServicePort,
		GossipPort:        gossipPort,
		ConfigFilePath:    fmt.Sprintf("%s/%s", configPath, configFile),
		BootstrapFilePath: fmt.Sprintf("%s/%s", bootstrapPath, bootstrapFile),
		GossipFilePath:    fmt.Sprintf("%s/%s", gossipPath, gossipFile),
	})
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ls.Namespace,
			Name:      configMapName(ls),
			Labels:    common.SubResourceLabels(ls),
		},
		Data: map[string]string{
			configFile: s,
			entrypoint: buff.String(),
		},
	}, nil
}

func HaKeeperAdds(ls *v1alpha1.LogSet) []string {
	// TODO: consider hole in asts ordinals
	var seeds []string
	for i := int32(0); i < ls.Spec.Replicas; i++ {
		podName := fmt.Sprintf("%s-%d", stsName(ls), i)
		seeds = append(seeds, fmt.Sprintf("%s.%s.%s.svc:%d", podName, headlessSvcName(ls), ls.Namespace, logServicePort))
	}
	return seeds
}

func gossipSeeds(ls *v1alpha1.LogSet, sts *kruisev1.StatefulSet) []string {
	var seeds []string
	r := *sts.Spec.Replicas
	i := 0
	for count := int32(0); count < r; i++ {
		if slices.Contains(sts.Spec.ReserveOrdinals, i) {
			// skip reserve ordinals
			continue
		}
		podName := fmt.Sprintf("%s-%d", stsName(ls), i)
		seeds = append(seeds, fmt.Sprintf("%s.%s.%s.svc:%d", podName, headlessSvcName(ls), ls.Namespace, gossipPort))
		// a valid replica found, count it
		count++
	}
	return seeds
}

func deploymentID(ls *v1alpha1.LogSet) uint64 {
	return xxhash.Sum64String(ls.Name) >> 1
}

func configMapName(ls *v1alpha1.LogSet) string {
	return resourceName(ls) + "-config"
}

func gossipConfigMapName(ls *v1alpha1.LogSet) string {
	return resourceName(ls) + "-gossip"
}
