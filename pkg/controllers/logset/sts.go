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

package logset

import (
	"fmt"

	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/openkruise/kruise-api/apps/pub"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	configVolume = "config"
	configPath   = "/etc/logservice"
	gossipVolume = "gossip"
	gossipPath   = "/etc/gossip"

	bootstrapVolume = "bootstrap"
	bootstrapPath   = "/etc/bootstrap"

	PodNameEnvKey     = "POD_NAME"
	HeadlessSvcEnvKey = "HEADLESS_SERVICE_NAME"
	NamespaceEnvKey   = "NAMESPACE"
	PodIPEnvKey       = "POD_IP"

	logSuffix = "-log"
)

// syncReplicas controls the real replicas field of the logset pods
func syncReplicas(ls *v1alpha1.LogSet, sts *kruisev1.StatefulSet) {
	sts.Spec.Replicas = &ls.Spec.Replicas
}

// syncPodMeta controls the metadata of the underlying logset pods, update meta might not need to trigger rolling-update
func syncPodMeta(ls *v1alpha1.LogSet, sts *kruisev1.StatefulSet) {
	common.SyncPodMeta(&sts.Spec.Template.ObjectMeta, &ls.Spec.PodSet)
	ls.Spec.Overlay.OverlayPodMeta(&sts.Spec.Template.ObjectMeta)
}

// syncPodSpec controls pod spec of the underlying logset pods
func syncPodSpec(ls *v1alpha1.LogSet, specRef *corev1.PodSpec) {
	// we should sync the spec as fine-grained as possible since not all fields in spec are managed by
	// current controller (e.g. some fields might be populated by the defaulting logic of api-server)
	mainRef := util.FindFirst(specRef.Containers, func(c corev1.Container) bool {
		return c.Name == v1alpha1.ContainerMain
	})
	if mainRef == nil {
		mainRef = &corev1.Container{Name: v1alpha1.ContainerMain}
	}
	mainRef.Image = ls.Spec.Image
	mainRef.Resources = ls.Spec.Resources
	mainRef.Command = []string{"/bin/sh", fmt.Sprintf("%s/%s", configPath, entrypoint)}
	mainRef.Args = ls.Spec.ServiceArgs
	mainRef.VolumeMounts = []corev1.VolumeMount{
		{Name: common.DataVolume, MountPath: common.DataPath},
		{Name: bootstrapVolume, ReadOnly: true, MountPath: bootstrapPath},
		{Name: configVolume, ReadOnly: true, MountPath: configPath},
		{Name: gossipVolume, ReadOnly: true, MountPath: gossipPath},
	}
	mainRef.Env = []corev1.EnvVar{
		util.FieldRefEnv(PodNameEnvKey, "metadata.name"),
		util.FieldRefEnv(NamespaceEnvKey, "metadata.namespace"),
		util.FieldRefEnv(PodIPEnvKey, "status.podIP"),
		{Name: HeadlessSvcEnvKey, Value: headlessSvcName(ls)},
	}
	if v1alpha1.GateInplaceConfigmapUpdate.Enabled(ls.Spec.GetOperatorVersion()) {
		mainRef.Env = append(mainRef.Env, util.FieldRefEnv(common.ConfigSuffixEnvKey, fmt.Sprintf("metadata.annotations['%s']", common.ConfigSuffixAnno)))
	}
	memLimitEnv := common.GoMemLimitEnv(ls.Spec.MemoryLimitPercent, ls.Spec.Resources.Limits.Memory(), ls.Spec.Overlay)
	if memLimitEnv != nil {
		mainRef.Env = append(mainRef.Env, *memLimitEnv)
	}

	//if ls.Spec.DNSBasedIdentity {
	//	mainRef.Env = append(mainRef.Env, corev1.EnvVar{Name: "HOSTNAME_UUID", Value: "y"})
	//}
	ls.Spec.Overlay.OverlayMainContainer(mainRef)

	specRef.Containers = []corev1.Container{*mainRef}
	specRef.Volumes = []corev1.Volume{{
		// bootstrap configmap is immutable before the bootstrap is complete and no rolling-update
		// is required when we clean its content after bootstrap completes
		Name:         bootstrapVolume,
		VolumeSource: util.ConfigMapVolume(bootstrapConfigMapName(ls)),
	}, {
		// gossip configmap will be changed when cluster is scaled, we don't want to rolling-update
		// the cluster when such change happens
		Name:         gossipVolume,
		VolumeSource: util.ConfigMapVolume(gossipConfigMapName(ls)),
	}}
	specRef.ReadinessGates = []corev1.PodReadinessGate{{
		ConditionType: pub.InPlaceUpdateReady,
	}}
	specRef.NodeSelector = ls.Spec.NodeSelector
	common.SetStorageProviderConfig(ls.Spec.SharedStorage, specRef)
	common.SyncTopology(ls.Spec.TopologyEvenSpread, specRef, &metav1.LabelSelector{MatchLabels: common.SubResourceLabels(ls)})
	ls.Spec.Overlay.OverlayPodSpec(specRef)
	common.SetupMemoryFsVolume(specRef, ls.Spec.MemoryFsSize)
}

// syncPersistentVolumeClaim controls the persistent volume claim of underlying pods
func syncPersistentVolumeClaim(ls *v1alpha1.LogSet, sts *kruisev1.StatefulSet) {
	dataPVC := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.DataVolume,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: ls.Spec.Volume.Size,
				},
			},
			StorageClassName: ls.Spec.Volume.StorageClassName,
		},
	}
	tpls := []corev1.PersistentVolumeClaim{dataPVC}
	ls.Spec.Overlay.AppendVolumeClaims(&tpls)
	sts.Spec.VolumeClaimTemplates = tpls
}

// syncStatefulSetSpec syncs the statefulset to the current desired state
func syncStatefulSetSpec(ls *v1alpha1.LogSet, sts *kruisev1.StatefulSet) {
	switch ls.Spec.GetPVCRetentionPolicy() {
	case v1alpha1.PVCRetentionPolicyDelete:
		sts.Spec.PersistentVolumeClaimRetentionPolicy = &kruisev1.StatefulSetPersistentVolumeClaimRetentionPolicy{
			WhenDeleted: kruisev1.DeletePersistentVolumeClaimRetentionPolicyType,
			WhenScaled:  kruisev1.DeletePersistentVolumeClaimRetentionPolicyType,
		}
	case v1alpha1.PVCRetentionPolicyRetain:
		sts.Spec.PersistentVolumeClaimRetentionPolicy = &kruisev1.StatefulSetPersistentVolumeClaimRetentionPolicy{
			WhenDeleted: kruisev1.RetainPersistentVolumeClaimRetentionPolicyType,
			WhenScaled:  kruisev1.RetainPersistentVolumeClaimRetentionPolicyType,
		}
	}
}

// buildStatefulSet build the initial StatefulSet object for the given logset
func buildStatefulSet(ls *v1alpha1.LogSet, headlessSvc *corev1.Service) *kruisev1.StatefulSet {
	sts := &kruisev1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ls.Namespace,
			Name:      stsName(ls),
			Labels:    common.SubResourceLabels(ls),
		},
		Spec: kruisev1.StatefulSetSpec{
			ServiceName: headlessSvc.Name,
			UpdateStrategy: kruisev1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &kruisev1.RollingUpdateStatefulSetStrategy{
					PodUpdatePolicy: kruisev1.InPlaceIfPossiblePodUpdateStrategyType,
				},
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{
				MatchLabels: common.SubResourceLabels(ls),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      common.SubResourceLabels(ls),
					Annotations: map[string]string{},
				},
			},
		},
	}
	syncStatefulSetSpec(ls, sts)
	return sts
}

// buildHeadlessSvc build the initial headless service object for the given logset
func buildHeadlessSvc(ls *v1alpha1.LogSet) *corev1.Service {
	return common.HeadlessServiceTemplate(ls, headlessSvcName(ls))
}

func stsName(ls *v1alpha1.LogSet) string {
	return resourceName(ls)
}

func headlessSvcName(ls *v1alpha1.LogSet) string {
	return resourceName(ls) + "-headless"
}

func resourceName(ls *v1alpha1.LogSet) string {
	return ls.Name + logSuffix
}
