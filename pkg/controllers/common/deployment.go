package common

import (
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DeploymentTemplate(obj client.Object, name string, spec appsv1.DeploymentSpec) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta:   DeploymentTypeMeta(),
		ObjectMeta: ObjMetaTemplate(obj, name),
		Spec:       spec,
	}
}

func DeploymentTypeMeta() metav1.TypeMeta {
	return metav1.TypeMeta{
		Kind:       "Deployment",
		APIVersion: "apps/v1",
	}
}
