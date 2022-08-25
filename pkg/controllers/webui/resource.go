package webui

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func BuildWebUI(mo *v1alpha1.MatrixOneCluster) *appsv1.Deployment {
	spec := appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: common.SubResourceLabels(mo),
		},
		Strategy: appsv1.DeploymentStrategy{
			Type:          "RollingUpdate",
			RollingUpdate: getRollingUpdateStrategy(),
		},
	}
	return common.DeploymentTemplate(mo, "webui", spec)
}

func getRollingUpdateStrategy() *appsv1.RollingUpdateDeployment {
	//var nil *int32 = nil
	//if nodeSpec.MaxSurge != nil || nodeSpec.MaxUnavailable != nil {
	//	return &appsv1.RollingUpdateDeployment{
	//		MaxUnavailable: &intstr.IntOrString{
	//			IntVal: *nodeSpec.MaxUnavailable,
	//		},
	//		MaxSurge: &intstr.IntOrString{
	//			IntVal: *nodeSpec.MaxSurge,
	//		},
	//	}
	//}
	return &appsv1.RollingUpdateDeployment{
		MaxUnavailable: &intstr.IntOrString{
			IntVal: int32(25),
		},
		MaxSurge: &intstr.IntOrString{
			IntVal: int32(25),
		},
	}
}
