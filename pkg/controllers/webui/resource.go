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

package webui

import (
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/cnset"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/openkruise/kruise-api/apps/pub"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	serverPort   = 8007
	frontendPort = 8001

	webuiRepo     = "matrixorigin/dashboard"
	frontendImage = webuiRepo + ":frontend-0.1.0"
	backendImage  = webuiRepo + ":backend-0.1.0"

	// TODO: using credential by generated
	rootUser     = "dump"
	rootPassword = "111"
)

func syncReplicas(wi *v1alpha1.WebUI, dp *appsv1.Deployment) {
	dp.Spec.Replicas = &wi.Spec.Replicas
}

func syncPodMeta(wi *v1alpha1.WebUI, dp *appsv1.Deployment) {
	wi.Spec.Overlay.OverlayPodMeta(&dp.Spec.Template.ObjectMeta)
}

func syncPodSpec(wi *v1alpha1.WebUI, dp *appsv1.Deployment) {
	specRef := &dp.Spec.Template.Spec
	var updateStrategy appsv1.DeploymentStrategy

	maxUnavailable := &intstr.IntOrString{}
	maxSurge := &intstr.IntOrString{}
	if wi.Spec.UpdateStrategy.MaxUnavailable != nil {
		maxUnavailable = wi.Spec.UpdateStrategy.MaxUnavailable
	}
	if wi.Spec.UpdateStrategy.MaxSurge != nil {
		maxSurge = wi.Spec.UpdateStrategy.MaxSurge
	}

	updateStrategy = appsv1.DeploymentStrategy{
		Type: "RollingUpdate",
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxUnavailable: maxUnavailable,
			MaxSurge:       maxSurge,
		},
	}

	bi := buildBackendService(wi)
	fi := buildFrontendService(wi)

	dp.Spec.Strategy = updateStrategy
	dp.Spec.Replicas = &wi.Spec.Replicas

	specRef.Containers = []corev1.Container{bi, fi}
	specRef.ReadinessGates = []corev1.PodReadinessGate{{
		ConditionType: pub.InPlaceUpdateReady,
	}}
	specRef.NodeSelector = wi.Spec.NodeSelector
	common.SyncTopology(wi.Spec.TopologyEvenSpread, specRef, dp.Spec.Selector)
	wi.Spec.Overlay.OverlayPodSpec(specRef)
}

func buildFrontendService(wi *v1alpha1.WebUI) corev1.Container {
	c := corev1.Container{
		Name:  getFrontendName(wi),
		Image: frontendImage,
		Ports: []corev1.ContainerPort{
			{
				Name:          "web",
				ContainerPort: frontendPort,
			},
		},
	}

	return c
}

func buildBackendService(wi *v1alpha1.WebUI) corev1.Container {
	volumeMountsList := []corev1.VolumeMount{
		{
			Name:      common.ConfigVolume,
			ReadOnly:  true,
			MountPath: common.ConfigPath,
		},
	}
	c := corev1.Container{
		Name:  getBackendName(wi),
		Image: backendImage,
		Command: []string{
			"/mocloud-metric-service",
			"-c",
		},
		Args: []string{
			common.ConfigPath + "/config.toml",
		},
		VolumeMounts: volumeMountsList,
		Ports: []corev1.ContainerPort{
			{
				Name:          "service",
				ContainerPort: serverPort,
			},
		},
	}
	return c
}

func syncPods(ctx *recon.Context[*v1alpha1.WebUI], dp *appsv1.Deployment) error {
	cm, err := buildConfigMap(ctx.Obj)
	if err != nil {
		return err
	}

	syncPodMeta(ctx.Obj, dp)
	syncPodSpec(ctx.Obj, dp)

	return common.SyncConfigMap(ctx, &dp.Spec.Template.Spec, cm, ctx.Obj.Spec.GetOperatorVersion())
}

func syncServiceType(wi *v1alpha1.WebUI, svc *corev1.Service) {
	svc.Spec.Type = wi.Spec.ServiceType
}

func buildWebUI(wi *v1alpha1.WebUI) *appsv1.Deployment {
	return common.DeploymentTemplate(wi, webUIName(wi))
}

func buildConfigMap(wi *v1alpha1.WebUI) (*corev1.ConfigMap, error) {
	conf := wi.Spec.Config
	if conf == nil {
		conf = v1alpha1.NewTomlConfig(map[string]interface{}{})
	}
	conf.Set([]string{"db", "host"}, getCNService(wi))
	conf.Set([]string{"db", "port"}, cnset.CNSQLPort)
	conf.Set([]string{"db", "username"}, rootUser)
	conf.Set([]string{"db", "password"}, rootPassword)
	conf.Set([]string{"log", "level"}, "info")
	conf.Set([]string{"log", "format"}, "console")
	conf.Set([]string{"server", "host"}, common.AnyIP)
	conf.Set([]string{"server", "port"}, serverPort)
	conf.Set([]string{"server", "tokenEffectiveTime"}, 2)
	conf.Set([]string{"server", "mode"}, "playground")

	s, err := conf.ToString()
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: common.ObjMetaTemplate(wi, configMapName(wi)),
		Data: map[string]string{
			common.ConfigFile: s,
		},
	}, nil

}

func buildService(wi *v1alpha1.WebUI) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: common.ObjMetaTemplate(wi, webUIName(wi)),
		Spec: corev1.ServiceSpec{
			Type:     wi.Spec.ServiceType,
			Selector: common.SubResourceLabels(wi),
			Ports: []corev1.ServicePort{
				{
					Name: "web",
					Port: frontendPort,
				},
			},
		},
	}
}

func getFrontendName(wi *v1alpha1.WebUI) string {
	return wi.Name + "-frontend"
}

func getBackendName(wi *v1alpha1.WebUI) string {
	return wi.Name + "-backend"
}

func getCNService(wi *v1alpha1.WebUI) string {
	return wi.Name + "-tp-cn"
}
