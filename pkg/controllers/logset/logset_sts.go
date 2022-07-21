package logset

import (
	"fmt"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultGraceSeconds = 30

	dataVolume   = "data"
	dataPath     = "/var/lib/logservice"
	configVolume = "config"
	configPath   = "/etc/logservice"
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
	main := corev1.Container{
		Name:      v1alpha1.ContainerMain,
		Image:     fmt.Sprintf(ls.Spec.Image),
		Resources: ls.Spec.Resources,
		VolumeMounts: []corev1.VolumeMount{{
			Name:      dataVolume,
			MountPath: dataPath,
		}},
	}
	ls.Spec.Overlay.OverlayMainContainer(&main)
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{main},
		Volumes:    []corev1.Volume{},
	}
	// TODO(aylei): attach configmap hash to pod annotation to trigger rolling-update on config change
	// TODO(aylei): if external configmap is not provided, generate a default one
	if ls.Spec.ConfigMap != nil {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name: configVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: ls.Spec.ConfigMap.Name,
					},
				},
			},
		})
		main.VolumeMounts = append(main.VolumeMounts, corev1.VolumeMount{
			Name:      configVolume,
			MountPath: configPath,
		})
	}
	ls.Spec.Overlay.OverlayPodSpec(&podSpec)
	sts.Spec.Template.Spec = podSpec
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
		},
		// TODO(aylei): ports definition
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  common.SubResourceLabels(ls),
		},
	}
}

func stsName(ls *v1alpha1.LogSet) string {
	return ls.Name
}

func headlessSvcName(ls *v1alpha1.LogSet) string {
	return ls.Name + "-headless"
}
