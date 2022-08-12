// Copyright 2022 Matrix Origin
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

package dnset

import (
	"bytes"
	"fmt"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/logset"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"text/template"
)

var startScriptTpl = template.Must(template.New("dnservice-start-script").Parse(`
#!/bin/sh
set -eu

POD_NAME=${POD_NAME:-$HOSTNAME}
ADDR="${POD_NAME}.${HEADLESS_SERVICE_NAME}.${NAMESPACE}.svc"
ORDINAL=${POD_NAME##*-}
UUID=$(printf '00000000-0000-0000-0000-%012x' ${ORDINAL})
conf=$(mktemp)

bc=$(mktemp)
cat <<EOF > ${bc}
uuid = "${UUID}"
service-address = "0.0.0.0:{{ .DNServicePort }}"
listen-address = "${ADDR}:{{ .DNServicePort }}"
EOF

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
	HAKeeperPort   int
	DNServicePort  int
	FileService    []fileService
	ConfigFilePath string
}

type fileService struct{}

// buildDNSetConfigMap return dn set configmap
func buildDNSetConfigMap(dn *v1alpha1.DNSet) (*corev1.ConfigMap, error) {
	conf := dn.Spec.Config

	if conf == nil {
		conf = v1alpha1.NewTomlConfig(map[string]interface{}{})
	}

	// detail: https://github.com/matrixorigin/matrixone/blob/main/pkg/cnservice/types.go
	conf.Set([]string{"service-type"}, ServiceTypeDN)
	conf.Set([]string{"dn", "Txn", "Storage"}, getStorageConfig(dn))
	conf.Set([]string{"fileservice"}, getLocalStorageConfig(dn))
	conf.Set([]string{"fileservice"}, getSharedStorageConfig(dn))
	conf.Set([]string{"dn", "hakeeper-client", "service-addresses"}, getHaKeeperAdds(dn))
	s, err := conf.ToString()
	if err != nil {
		return nil, err
	}

	buff := new(bytes.Buffer)
	err = startScriptTpl.Execute(buff, &model{
		FileService:    getFileService(dn),
		ConfigFilePath: fmt.Sprintf("%s/%s", configPath, ConfigFile),
	})

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dn.Namespace,
			Name:      utils.GetConfigName(dn),
			Labels:    common.SubResourceLabels(dn),
		},
		Data: map[string]string{
			ConfigFile: s,
			Entrypoint: buff.String(),
		},
	}, nil
}

func getSharedStorageConfig(dn *v1alpha1.DNSet) string {
	return ""
}

func getLocalStorageConfig(dn *v1alpha1.DNSet) string {
	return ""
}

func getHaKeeperAdds(dn *v1alpha1.DNSet) []string {
	ls := &v1alpha1.LogSet{}

	var seeds []string
	for i := int32(0); i < ls.Spec.Replicas; i++ {
		podName := fmt.Sprintf("%s-%d", utils.GetName(ls), i)
		seeds = append(seeds, fmt.Sprintf("%s.%s.%s.svc:%d", podName,
			utils.GetHeadlessSvcName(ls),
			utils.GetNamespace(ls), logset.LogServicePort))
	}
	return seeds
}

func getStorageConfig(dn *v1alpha1.DNSet) []string {
	return []string{}
}

func getFileService(dn *v1alpha1.DNSet) []fileService {
	return []fileService{}
}
