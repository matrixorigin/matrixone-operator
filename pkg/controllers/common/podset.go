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
	"fmt"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/openkruise/kruise-api/apps/pub"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SyncMOPodTask struct {
	PodSet          *v1alpha1.PodSet
	TargetTemplate  *corev1.PodTemplateSpec
	ConfigMap       *corev1.ConfigMap
	KubeCli         recon.KubeClient
	StorageProvider *v1alpha1.SharedStorageProvider

	// optional
	MutateContainer func(c *corev1.Container)
	MutatePod       func(p *corev1.PodTemplateSpec)
}

// SyncMOPod execute the given SyncMOPodTask which keeps the pod spec update to date
func SyncMOPod(t *SyncMOPodTask) error {
	syncPodTemplate(t)
	if err := SyncConfigMap(t.KubeCli, &t.TargetTemplate.Spec, t.ConfigMap); err != nil {
		return errors.Wrap(err, "sync configmap")
	}
	return nil
}

// syncPodTemplate apply the podset spec to a pod template
func syncPodTemplate(t *SyncMOPodTask) {
	specRef := &t.TargetTemplate.Spec

	mainRef := util.FindFirst(specRef.Containers, func(c corev1.Container) bool {
		return c.Name == v1alpha1.ContainerMain
	})
	if mainRef == nil {
		mainRef = &corev1.Container{Name: v1alpha1.ContainerMain}
	}
	syncMainContainer(t.PodSet, mainRef, t.MutateContainer)

	specRef.Containers = []corev1.Container{*mainRef}
	specRef.ReadinessGates = []corev1.PodReadinessGate{{
		ConditionType: pub.InPlaceUpdateReady,
	}}
	p := t.PodSet
	specRef.NodeSelector = p.NodeSelector
	SyncTopology(p.TopologyEvenSpread, specRef, &metav1.LabelSelector{MatchLabels: t.TargetTemplate.Labels})
	if t.MutatePod != nil {
		t.MutatePod(t.TargetTemplate)
	}
	if t.StorageProvider != nil {
		SetStorageProviderConfig(*t.StorageProvider, specRef)
	}
	p.Overlay.OverlayPodMeta(&t.TargetTemplate.ObjectMeta)
	p.Overlay.OverlayPodSpec(specRef)
}

func syncMainContainer(p *v1alpha1.PodSet, c *corev1.Container, mutateFn func(c *corev1.Container)) {
	c.Image = p.Image
	c.Resources = p.Resources
	c.Args = p.ServiceArgs
	c.Command = []string{"/bin/sh", fmt.Sprintf("%s/%s", ConfigPath, Entrypoint)}
	c.Env = []corev1.EnvVar{
		util.FieldRefEnv(PodNameEnvKey, "metadata.name"),
		util.FieldRefEnv(NamespaceEnvKey, "metadata.namespace"),
	}
	if p.DNSBasedIdentity {
		c.Env = append(c.Env, corev1.EnvVar{Name: "HOSTNAME_UUID", Value: "y"})
	}
	c.VolumeMounts = []corev1.VolumeMount{
		{
			Name:      ConfigVolume,
			ReadOnly:  true,
			MountPath: ConfigPath,
		},
	}
	if mutateFn != nil {
		mutateFn(c)
	}
	p.Overlay.OverlayMainContainer(c)
}
