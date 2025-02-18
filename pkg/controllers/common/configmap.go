// Copyright 2025 Matrix Origin
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
	"strings"

	"github.com/blang/semver/v4"
	"github.com/cespare/xxhash"
	"github.com/go-errors/errors"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
func SyncConfigMap(kubeCli recon.KubeClient, podSpec *corev1.PodSpec, cm *corev1.ConfigMap, operatorVersion semver.Version) error {
	var currentCmName string
	var desiredName string
	var err error
	vp := util.FindFirst(podSpec.Volumes, util.WithVolumeName("config"))
	if vp != nil {
		currentCmName = vp.Name
	}
	if v1alpha1.GateInplaceConfigmapUpdate.Enabled(operatorVersion) {
		desiredName, err = ensureConfigMap(kubeCli, cm)
	} else {
		desiredName, err = ensureConfigMapLegacy(kubeCli, currentCmName, cm)
	}
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
			} else {
				// log roll-out event to track configmap changes
				klog.Infof("config key %s is not in use, will be deleted. configmap %s/%s, value: %s",
					key, c.Namespace, c.Name, v)
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

// Deprecated: use ensureConfigMap instead
func ensureConfigMapLegacy(kubeCli recon.KubeClient, currentCm string, desired *corev1.ConfigMap) (string, error) {
	c := desired.DeepCopy()
	if err := addConfigMapDigest(c); err != nil {
		return "", errors.Wrap(err, 0)
	}
	// config digest not changed
	if c.Name == currentCm {
		return currentCm, nil
	}
	// otherwise ensure the configmap exists
	err := util.Ignore(apierrors.IsAlreadyExists, kubeCli.CreateOwned(c))
	if err != nil {
		return "", errors.Wrap(err, 0)
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
