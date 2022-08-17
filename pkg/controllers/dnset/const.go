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
	"text/template"
)

const (
	dnPortName = "dn-service"
)

// dn service entrypoint script
var startScriptTpl = template.Must(template.New("dn-start-script").Parse(`
#!/bin/sh
set -eu
POD_NAME=${POD_NAME:-$HOSTNAME}
ADDR="${POD_NAME}.${HEADLESS_SERVICE_NAME}.${NAMESPACE}.svc"
ORDINAL=${POD_NAME##*-}
UUID=$(printf '00000000-0000-0000-0000-1%011x' ${ORDINAL})
conf=$(mktemp)
bc=$(mktemp)
cat <<EOF > ${bc}
uuid = "${UUID}"
service-address = "${ADDR}:{{ .DNServicePort }}"
EOF
# build instance config
sed "/\[dn\]/r ${bc}" {{ .ConfigFilePath }} > ${conf}

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

touch /var/lib/matrixone/data/thisisalocalfileservicedir

echo "/mo-service -cfg ${conf}"
exec /mo-service -cfg ${conf}
`))
