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
package webui

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/openkruise/kruise-api/apps/pub"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func syncReplicas(wi *v1alpha1.WebUI, dp *appsv1.Deployment) {
	dp.Spec.Replicas = &wi.Spec.Replicas
}

func syncPodMeta(wi *v1alpha1.WebUI, dp *appsv1.Deployment) {
	wi.Spec.Overlay.OverlayPodMeta(&dp.Spec.Template.ObjectMeta)
}

func syncPodSpec(wi *v1alpha1.WebUI, dp *appsv1.Deployment) {
	main := corev1.Container{
		Image:     wi.Spec.Image,
		Name:      v1alpha1.ContainerMain,
		Resources: wi.Spec.Resources,
	}

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{main},
		ReadinessGates: []corev1.PodReadinessGate{{
			ConditionType: pub.InPlaceUpdateReady,
		}},
		NodeSelector: wi.Spec.NodeSelector,
	}

	updateStrategy := appsv1.DeploymentStrategy{
		Type:          "RollingUpdate",
		RollingUpdate: getRollingUpdateStrategy(wi),
	}

	common.SyncTopology(wi.Spec.TopologyEvenSpread, &podSpec)
	dp.Spec.Template.Spec = podSpec
	dp.Spec.Strategy = updateStrategy
	wi.Spec.Overlay.OverlayPodSpec(&podSpec)
}

func getRollingUpdateStrategy(wi *v1alpha1.WebUI) *appsv1.RollingUpdateDeployment {
	if wi.Spec.UpdateStrategy.MaxSurge != nil &&
		wi.Spec.UpdateStrategy.MaxUnavailable != nil {
		return &appsv1.RollingUpdateDeployment{
			MaxUnavailable: &intstr.IntOrString{
				IntVal: *wi.Spec.UpdateStrategy.MaxUnavailable,
			},
			MaxSurge: &intstr.IntOrString{
				IntVal: *wi.Spec.UpdateStrategy.MaxSurge,
			},
		}
	}
	return &appsv1.RollingUpdateDeployment{
		MaxUnavailable: &intstr.IntOrString{
			IntVal: int32(25),
		},
		MaxSurge: &intstr.IntOrString{
			IntVal: int32(25),
		},
	}
}

func syncPods(ctx *recon.Context[*v1alpha1.WebUI], dp *appsv1.Deployment) {
	syncPodMeta(ctx.Obj, dp)
	syncPodSpec(ctx.Obj, dp)
}

func buildWebUI(wi *v1alpha1.WebUI) *appsv1.Deployment {
	return common.DeploymentTemplate(wi, webUIName(wi))
}

func buildService(wi *v1alpha1.WebUI) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: common.ObjMetaTemplate(wi, webUIName(wi)),
		Spec: corev1.ServiceSpec{
			Type:     wi.Spec.ServiceType,
			Selector: common.SubResourceLabels(wi),
			// TODO: webui service ports config
			Ports: []corev1.ServicePort{},
		},
	}
}
