package logset

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/cespare/xxhash"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	configFile = "logservice.toml"
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
UUID=$(printf '00000000-0000-0000-0000-0%011x' ${ORDINAL})
conf=$(mktemp)

bc=$(mktemp)
cat <<EOF > ${bc}
uuid = "${UUID}"
raft-address = "${ADDR}:{{ .RaftPort }}"
logservice-address = "${ADDR}:{{ .LogServicePort }}"
gossip-address = "${POD_IP}:{{ .GossipPort }}"
EOF

# build instance config
sed "/\[logservice\]/r ${bc}" {{ .ConfigFilePath }} > ${conf}

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
    if [[ ${elapseTime} -ge ${threshold} ]]; then
        echo "waiting for dns resolvable timeout" >&2 && exit 1
    fi
    if nslookup ${ADDR} 2>/dev/null; then
        break
    else
        echo "waiting pod dns name ${ADDR} resolvable" >&2
    fi
done

echo "/mo-service -cfg ${conf}"
exec /mo-service -cfg ${conf}
`))

type model struct {
	RaftPort          int
	LogServicePort    int
	GossipPort        int
	ConfigFilePath    string
	BootstrapFilePath string
}

// buildConfigMap build the configmap for log service
func buildConfigMap(ls *v1alpha1.LogSet) (*corev1.ConfigMap, error) {
	conf := ls.Spec.Config
	if conf == nil {
		conf = v1alpha1.NewTomlConfig(map[string]interface{}{})
	}
	// 1. build base config file
	conf.Set([]string{"service-type"}, serviceTypeLog)
	conf.Set([]string{"logservice", "deployment-id"}, deploymentId(ls))
	conf.Set([]string{"logservice", "gossip-seed-addresses"}, gossipSeeds(ls))
	conf.Set([]string{"hakeeper-client", "service-addresses"}, HaKeeperAdds(ls))
	// conf.Set([]string{"hakeeper-client", "discovery-address"}, fmt.Sprintf("%s:%d", discoverySvcAddress(ls), LogServicePort))
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

func gossipSeeds(ls *v1alpha1.LogSet) []string {
	// TODO: consider hole in asts ordinals
	var seeds []string
	for i := int32(0); i < ls.Spec.Replicas; i++ {
		podName := fmt.Sprintf("%s-%d", stsName(ls), i)
		seeds = append(seeds, fmt.Sprintf("%s.%s.%s.svc:%d", podName, headlessSvcName(ls), ls.Namespace, gossipPort))
	}
	return seeds
}

func deploymentId(ls *v1alpha1.LogSet) uint64 {
	return xxhash.Sum64String(ls.Name) >> 1
}

func configMapName(ls *v1alpha1.LogSet) string {
	return resourceName(ls) + "-config"
}
