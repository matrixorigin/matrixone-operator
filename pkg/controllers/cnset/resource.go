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
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	configFile = "cn-config.toml"
)

const (
	serviceType   = "cn"
	listenAddress = ""
	logLevel      = "debug"
	logFormatType = "json"
	logMaxSize    = "512"
	backendType   = "s3"
	hostSize      = 1000
	guestSize     = 2000
	operatorSize  = 3000
	batchRow      = 300
	batchSize     = 400
)

func buildHeadlessSvc(cn *v1alpha1.CNSet) *corev1.Service {
	svc := &corev1.Service{}

	return svc
}

func buildCNSet(cn *v1alpha1.DNSet) *kruise.CloneSet {
	cnCloneSet := &kruise.CloneSet{}
	return cnCloneSet
}

func buildCNSetCOnfigMap(cn *v1alpha1.CNSet) (*corev1.ConfigMap, error) {
	configMapName := cn.Name + "-config"
	dsCfg := cn.Spec.Config
	// detail: https://github.com/matrixorigin/matrixone/blob/main/pkg/dnservice/cfg.go
	if dsCfg == nil {
		dsCfg = v1alpha1.NewTomlConfig(map[string]interface{}{
			"service-type":   serviceType,
			"listen-address": listenAddress,
			"log": map[string]interface{}{
				"level":    logLevel,
				"format":   logFormatType,
				"max-size": logMaxSize,
			},
			"file-service": map[string]interface{}{
				"backend": backendType,
				"s3": map[string]interface{}{
					"endpoint":   "",
					"bucket":     "",
					"key-prefix": "",
				},
			},
			"pipeline": map[string]interface{}{
				"host-size":     hostSize,
				"guest-size":    guestSize,
				"operator-size": operatorSize,
				"batch-row":     batchRow,
				"batch-size":    batchSize,
			},
		})
	}
	s, err := dsCfg.ToString()
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: cn.Namespace,
			Labels:    common.SubResourceLabels(cn),
		},
		Data: map[string]string{
			configFile: s,
		},
	}, nil
}
