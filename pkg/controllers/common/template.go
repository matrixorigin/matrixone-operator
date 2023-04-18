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
	kruise "github.com/openkruise/kruise-api/apps/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DataVolume is the volume name of data PV
	DataVolume = "mo-data"
	// DataPath is the path where the data volume will be mounted to
	DataPath = "/var/lib/matrixone"
	// DataDir is the directory under data path that will be used to store the data of mo disk backend
	DataDir = "data"
	// S3CacheDir is the directory under data path that will be used as mo S3 FS cache
	S3CacheDir = "disk-cache"
	// ETLCacheDir is the directory under data path that will be used as mo ETL FS cache
	ETLCacheDir = "etl-cache"

	// InstanceLabelKey labels the cluster instance name of the resource
	InstanceLabelKey = "matrixorigin.io/instance"
	// ComponentLabelKey labels the component type of the resource
	ComponentLabelKey = "matrixorigin.io/component"
	// NamespaceLabelKey labels the owner namespace of cluster-scope resources
	NamespaceLabelKey = "matrixorigin.io/namespace"
	// MatrixoneClusterLabelKey labels pod generated in certain mo cluster
	MatrixoneClusterLabelKey = "matrixorigin.io/cluster"
	// ActionRequiredLabelKey labels the resource that need manual intervention
	ActionRequiredLabelKey = "matrixorigin.io/action-required"
	// ActionRequiredLabelValue is a dummy value that is used with ActionRequiredLabelKey
	ActionRequiredLabelValue = "True"
	// LogSetOwnerKey labels the owner of orphaned LogSet Pod that is left by failover
	LogSetOwnerKey = "matrixorigin.io/logset-owner"

	// PodNameEnvKey is the container environment variable to reflect the name of the Pod that runs the container
	PodNameEnvKey = "POD_NAME"
	// HeadlessSvcEnvKey is the container environment variable to reflect the headless service name of the Pod that runs the container
	HeadlessSvcEnvKey = "HEADLESS_SERVICE_NAME"
	// NamespaceEnvKey  is the container environment variable to reflect the namespace of the Pod that runs the container
	NamespaceEnvKey = "NAMESPACE"
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
func SyncTopology(domains []string, podSpec *corev1.PodSpec, selector *metav1.LabelSelector) {
	var constraints []corev1.TopologySpreadConstraint
	for _, domain := range domains {
		constraints = append(constraints, corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       domain,
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector:     selector,
		})
	}
	podSpec.TopologySpreadConstraints = constraints
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
