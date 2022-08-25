package mocluster

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildWebUI(mo *v1alpha1.MatrixOneCluster) *appsv1.Deployment {
	webui := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mo.Name + "-webui",
			Namespace: mo.Namespace,
			Labels:    common.SubResourceLabels(mo),
		},
		Spec: getWebUISpec(),
	}

	return webui
}

func getWebUISpec() appsv1.DeploymentSpec {
	deploySpec := appsv1.DeploymentSpec{
		Template: getWebUIPodTemplate(),
	}

	return deploySpec
}

func getWebUIPodTemplate() v1.PodTemplateSpec {
	return v1.PodTemplateSpec{}
}

func getWebUIPodSpec() v1.PodSpec {
	spec := v1.PodSpec{}

	return spec
}
