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

package common

import (
	"encoding/json"
	"fmt"
	"github.com/cespare/xxhash"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	// ConfigVolume is the volume name of configmap
	ConfigVolume = "config"
	// ConfigPath is the path where the config volume will be mounted to
	ConfigPath = "/etc/matrixone/config"
	// ConfigFile is the default config file name
	ConfigFile = "config.toml"
	// Entrypoint is the entrypoint of mo container
	Entrypoint = "start.sh"
)

// SyncConfigMap syncs the desired configmap for pods, which will cause rolling-update if the
// data of the configmap is changed
func SyncConfigMap(kubeCli recon.KubeClient, podSpec *corev1.PodSpec, cm *corev1.ConfigMap) error {
	var currentCmName string
	vp := util.FindFirst(podSpec.Volumes, util.WithVolumeName("config"))
	if vp != nil {
		currentCmName = vp.Name
	}
	// TODO(aylei): GC stale configmaps (maybe in another worker?)
	desiredName, err := ensureConfigMap(kubeCli, currentCmName, cm)
	if err != nil {
		return err
	}
	if vp != nil {
		// update existing config volume ref
		if vp.VolumeSource.ConfigMap == nil {
			return errors.New("config volume must be sourced by a ConfigMap")
		}
		vp.VolumeSource.ConfigMap.Name = desiredName
	} else {
		// insert new config volume ref
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name:         "config",
			VolumeSource: util.ConfigMapVolume(desiredName),
		})
	}
	return nil
}

// ensureConfigMap ensures the configmap exist in k8s
func ensureConfigMap(kubeCli recon.KubeClient, currentCm string, desired *corev1.ConfigMap) (string, error) {
	c := desired.DeepCopy()
	if err := addConfigMapDigest(c); err != nil {
		return "", err
	}
	// config digest not changed
	if c.Name == currentCm {
		return currentCm, nil
	}
	// otherwise ensure the configmap exists
	err := util.Ignore(apierrors.IsAlreadyExists, kubeCli.CreateOwned(c))
	if err != nil {
		return "", err
	}
	return c.Name, nil
}

func addConfigMapDigest(cm *corev1.ConfigMap) error {
	s, err := json.Marshal(cm.Data)
	if err != nil {
		return err
	}
	sum := xxhash.Sum64(s)
	suffix := fmt.Sprintf("%x", sum)[0:7]
	cm.Name = fmt.Sprintf("%s-%s", cm.Name, suffix)
	return nil
}
