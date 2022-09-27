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
package common

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cespare/xxhash"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	kruise "github.com/openkruise/kruise-api/apps/v1beta1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FileType string
type ServiceType string
type BackendType string

const (
	PodNameEnvKey     = "POD_NAME"
	HeadlessSvcEnvKey = "HEADLESS_SERVICE_NAME"
	NamespaceEnvKey   = "NAMESPACE"

	// DataPath is the default path where the data volume will be mounted to
	DataPath = "/var/lib/matrixone/data"
	// DataDir is the default directory under data path that will be used to store the data of mo disk backend
	DataDir       = "data"
	ConfigPath    = "/etc/matrixone/config"
	ConfigFile    = "config.toml"
	DataVolume    = "data"
	ConfigVolume  = "config"
	Entrypoint    = "start.sh"
	ListenAddress = "0.0.0.0"

	S3Service    FileType = "S3"
	LocalService FileType = "LOCAL"
	ETLService   FileType = "ETL"

	DiskBackendType = "DISK"
	S3BackendType   = "S3"
	MEMBackendType  = "MEM"

	MemoryEngine BackendType = "MEM"
	TAEEngine    BackendType = "TAE"

	DNServicePort = 41010
	CNServicePort = 6001

	InstanceLabelKey  = "matrixorigin.io/instance"
	ComponentLabelKey = "matrixorigin.io/component"
	// NamespaceLabelKey is the label key for cluster-scope resources
	NamespaceLabelKey = "matrixorigin.io/namespace"

	ActionRequiredLabelKey   = "matrixorigin.io/action-required"
	ActionRequiredLabelValue = "True"

	AWSAccessKeyID     = "AWS_ACCESS_KEY_ID"
	AWSSecretAccessKey = "AWS_SECRET_ACCESS_KEY"
)

// SubResourceLabels generate labels for sub-resources
func SubResourceLabels(owner client.Object) map[string]string {
	return map[string]string{
		NamespaceLabelKey: owner.GetNamespace(),
		InstanceLabelKey:  owner.GetName(),
		ComponentLabelKey: owner.GetObjectKind().GroupVersionKind().Kind,
	}
}

// SyncTopology syncs the topology even spread of PodSet to the underlying pods
func SyncTopology(domains []string, podSpec *corev1.PodSpec) {
	var constraints []corev1.TopologySpreadConstraint
	for _, domain := range domains {
		constraints = append(constraints, corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       domain,
			WhenUnsatisfiable: corev1.DoNotSchedule,
		})
	}
	podSpec.TopologySpreadConstraints = constraints
}

// SetStorageProviderConfig set inject configuration of storage provider to Pods
func SetStorageProviderConfig(sp v1alpha1.SharedStorageProvider, podSpec *corev1.PodSpec) {
	for i := range podSpec.Containers {
		if s3p := sp.S3; s3p != nil {
			if s3p.SecretRef != nil {
				for _, key := range []string{AWSAccessKeyID, AWSSecretAccessKey} {
					podSpec.Containers[i].Env = util.UpsertByKey(podSpec.Containers[i].Env, corev1.EnvVar{Name: key, ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: *s3p.SecretRef,
							Key:                  key,
						},
					}}, util.EnvVarKey)
				}
			}
		}
	}
}

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

// HeadlessServiceTemplate returns a headless service as template
// https://kubernetes.io/docs/concepts/services-networking/service/#headless-services
func HeadlessServiceTemplate(obj client.Object, name string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: ObjMetaTemplate(obj, name),
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  SubResourceLabels(obj),
			// Need to propagate SRV DNS records for the sts Pods
			// for the purpose of peer discovery
			PublishNotReadyAddresses: true,
		},
	}

}

// ObjMetaTemplate get object metadata
func ObjMetaTemplate[T client.Object](obj T, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   obj.GetNamespace(),
		Annotations: map[string]string{},
		Labels:      SubResourceLabels(obj),
	}
}

// StatefulSetTemplate return a kruise statefulset as template
func StatefulSetTemplate(obj client.Object, name string, svcName string) *kruise.StatefulSet {
	return &kruise.StatefulSet{
		ObjectMeta: ObjMetaTemplate(obj, name),
		Spec: kruise.StatefulSetSpec{
			ServiceName: svcName,
			UpdateStrategy: kruise.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &kruise.RollingUpdateStatefulSetStrategy{
					PodUpdatePolicy: kruise.InPlaceIfPossiblePodUpdateStrategyType,
				},
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{
				MatchLabels: SubResourceLabels(obj),
			},
			PersistentVolumeClaimRetentionPolicy: &kruise.StatefulSetPersistentVolumeClaimRetentionPolicy{
				WhenDeleted: kruise.DeletePersistentVolumeClaimRetentionPolicyType,
				WhenScaled:  kruise.DeletePersistentVolumeClaimRetentionPolicyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      SubResourceLabels(obj),
					Annotations: map[string]string{},
				},
			},
		},
	}
}

// DeploymentTemplate return a deployment as template
func DeploymentTemplate(obj client.Object, name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: ObjMetaTemplate(obj, name),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: SubResourceLabels(obj),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      SubResourceLabels(obj),
					Annotations: map[string]string{},
				},
			},
		},
	}

}

// PersistentVolumeClaimTemplate returns a persistent volume claim object
func PersistentVolumeClaimTemplate(size resource.Quantity, sc *string, name string) corev1.PersistentVolumeClaim {
	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: DataVolume,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: size,
				},
			},
			StorageClassName: sc,
		},
	}
}

// LocalFilesServiceConfig returns a local file service config
func LocalFilesServiceConfig(path string) map[string]interface{} {
	return map[string]interface{}{
		"name":     LocalService,
		"backend":  DiskBackendType,
		"data-dir": path,
	}
}

// S3FileServiceConfig returns an S3 file service config based on the shared storage provider
func S3FileServiceConfig(l *v1alpha1.LogSet) map[string]interface{} {
	return map[string]interface{}{
		"name":    S3Service,
		"backend": MEMBackendType,
	}
	// FIXME: use TAE and S3
	// return sharedFileServiceConfig(l, S3Service, "data")
}

// ETLFileServiceConfig returns an ETL file service config based on the shared storage provider
func ETLFileServiceConfig(l *v1alpha1.LogSet) map[string]interface{} {
	return map[string]interface{}{
		"name":     ETLService,
		"backend":  "DISK-ETL",
		"data-dir": "store",
	}
	// FIXME: use TAE and S3
	// return sharedFileServiceConfig(l, ETLService, "etl")
}

func sharedFileServiceConfig(l *v1alpha1.LogSet, name FileType, dir string) map[string]interface{} {
	m := map[string]interface{}{
		"name":    name,
		"backend": S3BackendType,
	}
	if s3 := l.Spec.SharedStorage.S3; s3 != nil {
		s3Config := map[string]interface{}{}
		if s3.Endpoint != "" {
			s3Config["endpoint"] = s3.Endpoint
		} else {
			s3Config["endpoint"] = "s3.us-west-2.amazonaws.com"
		}
		paths := strings.SplitN(s3.Path, "/", 2)
		s3Config["bucket"] = paths[0]
		keyPrefix := dir
		if len(paths) > 1 {
			keyPrefix = fmt.Sprintf("%s/%s", strings.Trim(paths[1], "/"), dir)
		}
		s3Config["key-prefix"] = keyPrefix
		m["s3"] = s3Config
	}
	return m
}
