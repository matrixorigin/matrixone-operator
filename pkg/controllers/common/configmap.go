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

package common

import (
	"fmt"
	"github.com/cespare/xxhash"
	"github.com/go-errors/errors"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
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
	vp := util.FindFirst(podSpec.Volumes, util.WithVolumeName("config"))
	desiredName, err := ensureConfigMap(kubeCli, cm)
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
func ensureConfigMap(kubeCli recon.KubeClient, desired *corev1.ConfigMap) (string, error) {
	c := desired.DeepCopy()
	old := &corev1.ConfigMap{}
	exist, err := kubeCli.Exist(client.ObjectKeyFromObject(c), old)
	if err != nil {
		return "", err
	}
	if exist {
		podList := &corev1.PodList{}
		err = kubeCli.List(podList, client.InNamespace(c.Namespace))
		if err != nil {
			return "", err
		}
		for key, v := range old.Data {
			if withDigest(key, v) && configInUse(key, podList.Items) {
				// append item that is still in use
				c.Data[key] = v
			}
		}
		err = kubeCli.Update(c)
	} else {
		err = kubeCli.CreateOwned(c)
	}
	if err != nil {
		return "", err
	}
	return c.Name, nil
}

func withDigest(key string, v string) bool {
	return strings.Contains(key, DataDigest([]byte(v)))
}

func configInUse(key string, podList []corev1.Pod) bool {
	for _, pod := range podList {
		s := pod.Annotations[ConfigSuffixAnno]
		if len(s) > 0 && strings.Contains(key, s) {
			return true
		}
	}
	return false
}

func DataDigest(data []byte) string {
	sum := xxhash.Sum64(data)
	return fmt.Sprintf("%x", sum)[0:7]
}
