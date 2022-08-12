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

package dnset

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/utils"
	"github.com/openkruise/kruise-api/apps/pub"
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// buildHeadlessSvc build the initial headless service object for the given dnset
func buildHeadlessSvc(dn *v1alpha1.DNSet) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dn.Namespace,
			Name:      utils.GetNamespace(dn),
			Labels:    common.SubResourceLabels(dn),
		},

		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Ports:     getDNServicePort(dn),
			Selector:  common.SubResourceLabels(dn),
		},
	}

	return svc

}

// buildSvc build dn pod service
func buildSvc(dn *v1alpha1.DNSet) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dn.Namespace,
			Name:      utils.GetSvcName(dn),
			Labels:    common.SubResourceLabels(dn),
		},

		Spec: corev1.ServiceSpec{
			Type:     dn.Spec.ServiceType,
			Ports:    getDNServicePort(dn),
			Selector: common.SubResourceLabels(dn),
		},
	}
	return svc
}

// buildDNSet return kruise CloneSet as dn resource
func buildDNSet(dn *v1alpha1.DNSet) *kruise.CloneSet {
	dnCloneSet := &kruise.CloneSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dn.Namespace,
			Name:      dn.Name,
		},
		Spec: kruise.CloneSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: common.SubResourceLabels(dn),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:        dn.Name,
					Namespace:   dn.Namespace,
					Labels:      common.SubResourceLabels(dn),
					Annotations: map[string]string{},
				},
			},
			ScaleStrategy:        getScaleStrategyConfig(dn),
			UpdateStrategy:       getUpdateStrategyConfig(dn),
			RevisionHistoryLimit: dn.Spec.RevisionHistoryLimit,
			MinReadySeconds:      dn.Spec.MinReadySeconds,
			Lifecycle:            dn.Spec.Lifecycle,
		},
	}

	return dnCloneSet
}

func syncPersistentVolumeClaim(dn *v1alpha1.DNSet, cloneSet *kruise.CloneSet) {
	dataPVC := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataVolume,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: dn.Spec.CacheVolume.Size,
				},
			},
			StorageClassName: dn.Spec.CacheVolume.StorageClassName,
		},
	}
	tpls := []corev1.PersistentVolumeClaim{dataPVC}
	dn.Spec.Overlay.AppendVolumeClaims(&tpls)
	cloneSet.Spec.VolumeClaimTemplates = tpls
}

func syncReplicas(ds *v1alpha1.DNSet, cs *kruise.CloneSet) {
	cs.Spec.Replicas = &ds.Spec.Replicas

}

func syncPodMeta(ds *v1alpha1.DNSet, cs *kruise.CloneSet) {
	ds.Spec.Overlay.OverlayPodMeta(&cs.Spec.Template.ObjectMeta)
}

func syncPodSpec(ds *v1alpha1.DNSet, cs *kruise.CloneSet) {
	main := corev1.Container{
		Name:      v1alpha1.ContainerMain,
		Image:     ds.Spec.Image,
		Resources: ds.Spec.Resources,
		Command: []string{
			"/mo-service",
		},
		Args: []string{
			"-cfg",
			configPath,
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: dataVolume, ReadOnly: true, MountPath: dataPath},
			{Name: configVolume, ReadOnly: true, MountPath: configPath},
		},
		Env: []corev1.EnvVar{{
			Name: PodNameEnvKey,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		}},
	}
	ds.Spec.Overlay.OverlayMainContainer(&main)
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{main},
		ReadinessGates: []corev1.PodReadinessGate{{
			ConditionType: pub.InPlaceUpdateReady,
		}},
	}
	common.SyncTopology(ds.Spec.TopologyEvenSpread, &podSpec)

	ds.Spec.Overlay.OverlayPodSpec(&podSpec)
	cs.Spec.Template.Spec = podSpec
}
