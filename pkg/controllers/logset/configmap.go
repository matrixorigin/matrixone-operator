package logset

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"github.com/cespare/xxhash"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConfigFile = "logservice.toml"
	Entrypoint = "start.sh"

	RaftPort       = 32000
	LogServicePort = 32001
	GossipPort     = 32002

	ServiceTypeLog = "LOG"
)

// Since HA requires instance-based heterogeneous configuration (e.g. instance UUID and advertised addresses), we need a start script to build these configurations based on
// the instance meta injected by k8s downward API
// TODO(aylei): add logservice topology labels
var startScriptTpl = template.Must(template.New("logservice-start-script").Parse(`
#!/bin/sh
set -euo pipefail

POD_NAME=${POD_NAME:-$HOSTNAME}
ADDR="${POD_NAME}.${HEADLESS_SERVICE_NAME}.${NAMESPACE}.svc"
ORDINAL=${POD_NAME##*-}
UUID=$(printf '00000000-0000-0000-0000-%012x' ${ORDINAL})
conf=$(mktemp /tmp/config.XXXX)
bc=$(mktemp /tmp/bc.XXXX)
cat <<EOF > ${bc}
uuid = "${UUID}"
raft-address = "${ADDR}:{{ .RaftPort }}"
logservice-address = "${ADDR}:{{ .LogServicePort }}"
gossip-address = "${ADDR}:{{ .GossipPort }}"
EOF

# build instance config
sed "/\[logservice\]/r ${bc}" {{ .ConfigFilePath }} > ${conf}

# append bootstrap config
cat {{ .BootstrapFilePath }} >> ${conf}

echo "/mo-service --config ${conf}"
exec /mo-service --config ${conf}
`))

type model struct {
	RaftPort         int
	LogServicePort   int
	GossipPort       int
	ConfigFilePath   string
	BoostrapFilePath string
}

// buildConfigMap build the configmap for log service
func buildConfigMap(ls *v1alpha1.LogSet) (*corev1.ConfigMap, error) {
	conf := ls.Spec.Config
	if conf == nil {
		conf = v1alpha1.NewTomlConfig(map[string]interface{}{})
	}
	// 1. build base config file
	conf.Set([]string{"service-type"}, ServiceTypeLog)
	conf.Set([]string{"logservice", "deployment-id"}, deploymentId(ls))
	conf.Set([]string{"logservice", "gossip-seed-addresses"}, gossipSeeds(ls))
	conf.Set([]string{"logservice", "HAKeeperClientConfig", "hakeeper-service-addresses"}, fmt.Sprintf("%s:d", discoverySvcAddress(ls), LogServicePort))
	s, err := conf.ToString()
	if err != nil {
		return nil, err
	}

	// 2. build the start script
	buff := new(bytes.Buffer)
	err = startScriptTpl.Execute(buff, &model{
		RaftPort:         RaftPort,
		LogServicePort:   LogServicePort,
		GossipPort:       GossipPort,
		ConfigFilePath:   fmt.Sprintf("%s/%s", configPath, ConfigFile),
		BoostrapFilePath: fmt.Sprintf("%s/%s", bootstrapPath, bootstrapFile),
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
			ConfigFile: s,
			Entrypoint: buff.String(),
		},
	}, nil
}

func gossipSeeds(ls *v1alpha1.LogSet) string {
	// TODO: consider hole in asts ordinals
	sb := strings.Builder{}
	for i := int32(0); i < ls.Spec.Replicas; i++ {
		if i != 0 {
			sb.WriteRune(';')
		}
		podName := fmt.Sprintf("%s-%d", stsName(ls), i)
		sb.WriteString(fmt.Sprintf("%s.%s:%d", podName, headlessSvcName(ls), GossipPort))
	}
	return sb.String()
}

func deploymentId(ls *v1alpha1.LogSet) uint64 {
	return xxhash.Sum64String(ls.Name)
}

func configMapName(ls *v1alpha1.LogSet) string {
	return ls.Name + "-config"
}
