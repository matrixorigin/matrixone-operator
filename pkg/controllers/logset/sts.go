package logset

import (
	"fmt"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	"github.com/openkruise/kruise-api/apps/pub"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	dataVolume   = "data"
	dataPath     = "/var/lib/logservice"
	configVolume = "config"
	configPath   = "/etc/logservice"

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
	ls.Spec.Overlay.OverlayPodMeta(&sts.Spec.Template.ObjectMeta)
}

// syncPodSpec controls pod spec of the underlying logset pods
func syncPodSpec(ls *v1alpha1.LogSet, sts *kruisev1.StatefulSet) {
	// we should sync the spec as fine-grained as possible since not all fields in spec are managed by
	// current controller (e.g. some fields might be populated by the defaulting logic of api-server)
	specRef := &sts.Spec.Template.Spec

	mainRef := util.FindFirst(specRef.Containers, func(c corev1.Container) bool {
		return c.Name == v1alpha1.ContainerMain
	})
	if mainRef == nil {
		mainRef = &corev1.Container{Name: v1alpha1.ContainerMain}
	}
	mainRef.Image = ls.Spec.Image
	mainRef.Resources = ls.Spec.Resources
	mainRef.Command = []string{"/bin/sh", fmt.Sprintf("%s/%s", configPath, entrypoint)}
	mainRef.VolumeMounts = []corev1.VolumeMount{
		{Name: dataVolume, MountPath: dataPath},
		{Name: bootstrapVolume, ReadOnly: true, MountPath: bootstrapPath},
		{Name: configVolume, ReadOnly: true, MountPath: configPath},
	}
	mainRef.Env = []corev1.EnvVar{
		util.FieldRefEnv(PodNameEnvKey, "metadata.name"),
		util.FieldRefEnv(NamespaceEnvKey, "metadata.namespace"),
		util.FieldRefEnv(PodIPEnvKey, "status.podIP"),
		{Name: HeadlessSvcEnvKey, Value: headlessSvcName(ls)},
	}
	ls.Spec.Overlay.OverlayMainContainer(mainRef)

	specRef.Containers = []corev1.Container{*mainRef}
	specRef.Volumes = []corev1.Volume{{
		// bootstrap configmap is immutable before the bootstrap is complete and no rolling-update
		// is required when we clean its content after bootstrap completes
		Name:         bootstrapVolume,
		VolumeSource: util.ConfigMapVolume(bootstrapConfigMapName(ls)),
	}}
	specRef.ReadinessGates = []corev1.PodReadinessGate{{
		ConditionType: pub.InPlaceUpdateReady,
	}}
	specRef.NodeSelector = ls.Spec.NodeSelector
	common.SyncTopology(ls.Spec.TopologyEvenSpread, specRef)
	ls.Spec.Overlay.OverlayPodSpec(specRef)
}

// syncPersistentVolumeClaim controls the persistent volume claim of underlying pods
func syncPersistentVolumeClaim(ls *v1alpha1.LogSet, sts *kruisev1.StatefulSet) {
	dataPVC := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataVolume,
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

// buildStatefulSet build the initial StatefulSet object for the given logset
func buildStatefulSet(ls *v1alpha1.LogSet, headlessSvc *corev1.Service) *kruisev1.StatefulSet {
	sts := &kruisev1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ls.Namespace,
			Name:      stsName(ls),
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
			PersistentVolumeClaimRetentionPolicy: &kruisev1.StatefulSetPersistentVolumeClaimRetentionPolicy{
				WhenDeleted: kruisev1.DeletePersistentVolumeClaimRetentionPolicyType,
				WhenScaled:  kruisev1.DeletePersistentVolumeClaimRetentionPolicyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      common.SubResourceLabels(ls),
					Annotations: map[string]string{},
				},
			},
		},
	}
	return sts
}

// buildHeadlessSvc build the initial headless service object for the given logset
func buildHeadlessSvc(ls *v1alpha1.LogSet) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ls.Namespace,
			Name:      headlessSvcName(ls),
			Labels:    common.SubResourceLabels(ls),
		},
		// TODO(aylei): ports definition
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  common.SubResourceLabels(ls),
		},
	}
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
