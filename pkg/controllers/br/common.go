// Copyright 2023 Matrix Origin
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

package br

import (
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const (
	defaultCMDRestPort = 8080
)

type JobObject interface {
	recon.Conditional
	client.Object

	GetTTL() time.Duration
	GetOverlay() *v1alpha1.Overlay
}

func buildJob(o JobObject, image string, command string, injectEnv func(c *corev1.Container)) *batchv1.Job {
	meta := common.ObjMetaTemplate(o, o.GetName())
	brContainer := corev1.Container{
		Name:  "br",
		Image: image,
	}
	if o.GetOverlay() != nil {
		o.GetOverlay().OverlayMainContainer(&brContainer)
	}
	brContainer.Command = []string{"/cmdrest", "--", command}
	injectEnv(&brContainer)
	tpl := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: meta.Labels,
		},
		Spec: corev1.PodSpec{
			Containers:    []corev1.Container{brContainer},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
	if o.GetOverlay() != nil {
		o.GetOverlay().OverlayPodMeta(&tpl.ObjectMeta)
		o.GetOverlay().OverlayPodSpec(&tpl.Spec)
	}
	job := &batchv1.Job{
		ObjectMeta: meta,
		Spec: batchv1.JobSpec{
			Parallelism:  pointer.Int32(1),
			Completions:  pointer.Int32(1),
			BackoffLimit: pointer.Int32(0),
			Template:     tpl,
		},
	}
	return job
}

func buildSvc(o client.Object) *corev1.Service {
	meta := common.ObjMetaTemplate(o, o.GetName())
	svc := &corev1.Service{
		ObjectMeta: meta,
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:       "rest",
				Port:       defaultCMDRestPort,
				TargetPort: intstr.FromInt(defaultCMDRestPort),
			}},
			Selector: meta.Labels,
		},
	}
	return svc
}
