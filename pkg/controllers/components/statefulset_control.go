// Copyright 2021 Matrix Origin
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

package components

import (
	"github.com/matrixorigin/matrixone-operator/pkg/apis/matrixone/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	dataPath       string = "/store"
	logPath        string = "/log"
	dataVolName    string = "data"
	logVolName     string = "log"
	serverPort     int32  = 6001
	addrRaftPort   int32  = 10000
	addrClientPort int32  = 20000
	rpcAddrPort    int32  = 30000
	clientPort     int32  = 40000
	peerPort       int32  = 50000
	raftPort       int32  = 20100
)

var (
	minReplicas int32 = 1
)

func MakeSts(moc *v1alpha1.MatrixoneCluster, ls map[string]string) (*appsv1.StatefulSet, error) {
	hServiceName := moc.Name + "-headless"
	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      moc.Name,
			Namespace: moc.Namespace,
			Labels:    ls,
		},
		Spec: makeStsSpec(moc, ls, hServiceName),
	}, nil

}

// hServiceName headless service name
func makeStsSpec(moc *v1alpha1.MatrixoneCluster, ls map[string]string, hServiceName string) appsv1.StatefulSetSpec {
	updateStrategy := utils.FirstNonNilValue(moc.Spec.UpdateStrategy, &appsv1.StatefulSetUpdateStrategy{}).(*appsv1.StatefulSetUpdateStrategy)

	initZero := int32(0)
	if moc.Spec.Replicas == nil {
		moc.Spec.Replicas = &minReplicas
	}

	if moc.Spec.Replicas != nil && *moc.Spec.Replicas < 0 {
		moc.Spec.Replicas = &initZero
	}

	stsSpec := appsv1.StatefulSetSpec{
		ServiceName: hServiceName,
		Selector: &metav1.LabelSelector{
			MatchLabels: ls,
		},
		Replicas: moc.Spec.Replicas,
		PodManagementPolicy: appsv1.PodManagementPolicyType(
			utils.FirstNonEmptyStr(utils.FirstNonEmptyStr(string(moc.Spec.PodManagementPolicy), string(moc.Spec.PodManagementPolicy)), string(appsv1.ParallelPodManagement))),
		UpdateStrategy:       *updateStrategy,
		Template:             makePodTemplate(moc, ls, hServiceName),
		VolumeClaimTemplates: getPersistentVolumeClaim(moc, ls),
	}

	return stsSpec

}

func makePodTemplate(moc *v1alpha1.MatrixoneCluster, ls map[string]string, hServiceName string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      ls,
			Annotations: moc.Spec.PodAnnotations,
		},
		Spec: makePodSpec(moc, hServiceName),
	}
}

func makePodSpec(moc *v1alpha1.MatrixoneCluster, hServiceName string) corev1.PodSpec {
	domain := ".svc.cluster.local"
	firstNode := moc.Name + "-0" + "." + hServiceName + "." + moc.Namespace + domain
	hServiceName = hServiceName + "." + moc.Namespace + domain

	spec := corev1.PodSpec{
		NodeSelector:     moc.Spec.NodeSelector,
		Tolerations:      moc.Spec.Tolerations,
		Affinity:         moc.Spec.Affinity,
		ImagePullSecrets: moc.Spec.ImagePullSecrets,
		DNSPolicy:        moc.Spec.DNSPolicy,
		DNSConfig:        moc.Spec.DNSConfig,
		Containers: []corev1.Container{
			{
				Name:            moc.Name,
				Image:           moc.Spec.Image,
				ImagePullPolicy: moc.Spec.ImagePullPolicy,
				Env: []corev1.EnvVar{
					{
						Name:  "FIRST_NODE",
						Value: firstNode,
					},
					{
						Name:  "SERVICE_NAME",
						Value: hServiceName,
					},
					moc.Spec.PodName,
				},
				Resources:      getResources(moc),
				LivenessProbe:  moc.Spec.LivenessProbe,
				ReadinessProbe: moc.Spec.ReadinessProbe,
				Command:        moc.Spec.Command,
				Ports: []corev1.ContainerPort{
					{
						Name:          "server",
						ContainerPort: serverPort,
					},
					{
						Name:          "addr-raft",
						ContainerPort: addrRaftPort,
					},
					{
						Name:          "addr-client",
						ContainerPort: addrClientPort,
					},
					{
						Name:          "rpc",
						ContainerPort: rpcAddrPort,
					},
					{
						Name:          "client",
						ContainerPort: clientPort,
					},
					{
						Name:          "peer",
						ContainerPort: peerPort,
					},
					{
						Name:          "raft",
						ContainerPort: raftPort,
					},
				},
				VolumeMounts: getVolumeMounts(moc),
			},
		},
		TerminationGracePeriodSeconds: moc.Spec.TerminationGracePeriodSeconds,
	}

	return spec
}

func getResources(moc *v1alpha1.MatrixoneCluster) corev1.ResourceRequirements {
	if moc.Spec.Requests == nil {
		moc.Spec.Requests = corev1.ResourceList{}
	}

	if moc.Spec.Limits == nil {
		moc.Spec.Limits = corev1.ResourceList{}
	}

	return corev1.ResourceRequirements{
		Requests: moc.Spec.Requests,
		Limits:   moc.Spec.Limits,
	}
}

func getVolumeMounts(moc *v1alpha1.MatrixoneCluster) []corev1.VolumeMount {
	volumeMount := []corev1.VolumeMount{
		{
			Name:      dataVolName,
			MountPath: dataPath,
		},
		{
			Name:      logVolName,
			MountPath: logPath,
		},
	}

	return volumeMount
}

func getPersistentVolumeClaim(moc *v1alpha1.MatrixoneCluster, ls map[string]string) []corev1.PersistentVolumeClaim {
	pvc := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dataVolName,
				Namespace: moc.Namespace,
				Labels:    ls,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					"ReadWriteOnce",
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(moc.Spec.DataVolCap),
					},
				},
				StorageClassName: moc.Spec.StorageClass,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      logVolName,
				Namespace: moc.Namespace,
				Labels:    ls,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					"ReadWriteOnce",
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(moc.Spec.LogVolCap),
					},
				},
				StorageClassName: moc.Spec.StorageClass,
			},
		},
	}

	return pvc

}

func MakeStatefulSetEmptyObj() *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
	}
}

func MakeStatefulSetListEmptyObj() *appsv1.StatefulSetList {
	return &appsv1.StatefulSetList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
	}
}
