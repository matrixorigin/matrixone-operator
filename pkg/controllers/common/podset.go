// Copyright 2025 Matrix Origin
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
	"strconv"

	"github.com/blang/semver/v4"
	"github.com/go-errors/errors"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/openkruise/kruise-api/apps/pub"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SyncMOPodTask struct {
	PodSet          *v1alpha1.PodSet
	TargetTemplate  *corev1.PodTemplateSpec
	ConfigMap       *corev1.ConfigMap
	KubeCli         recon.KubeClient
	StorageProvider *v1alpha1.SharedStorageProvider
	ConfigSuffix    string
	// optional
	MutateContainer func(c *corev1.Container)
	MutatePod       func(p *corev1.PodTemplateSpec)
}

// SyncPodMeta sync PodSet to pod object meta
func SyncPodMeta(meta *metav1.ObjectMeta, p *v1alpha1.PodSet) {
	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}
	if p.PromDiscoveredByPod() {
		meta.Annotations[PrometheusScrapeAnno] = "true"
		meta.Annotations[PrometheusPortAnno] = strconv.Itoa(MetricsPort)
	} else {
		delete(meta.Annotations, PrometheusScrapeAnno)
		delete(meta.Annotations, PrometheusPortAnno)
	}
	v, ok := p.GetSemVer()
	if ok {
		meta.Annotations[SemanticVersionAnno] = v.String()
	}
	ov := p.GetOperatorVersion()
	// backward compatible for old operator version
	if ov.GT(v1alpha1.FirstOpVersion) {
		meta.Annotations[v1alpha1.OperatorVersionAnno] = ov.String()
	}
}

// GetSemanticVersion returns the semantic of the target MO pod,
// if no version is parsed, a dummy version is returned
func GetSemanticVersion(meta *metav1.ObjectMeta) semver.Version {
	if anno, ok := meta.Annotations[SemanticVersionAnno]; ok {
		v, err := semver.Parse(anno)
		if err == nil {
			return v
		}
	}
	return v1alpha1.MinimalVersion
}

// SyncMOPod execute the given SyncMOPodTask which keeps the pod spec update to date
func SyncMOPod(t *SyncMOPodTask) error {
	syncPodTemplate(t)
	if err := SyncConfigMap(t.KubeCli, &t.TargetTemplate.Spec, t.ConfigMap, t.PodSet.GetOperatorVersion()); err != nil {
		return errors.WrapPrefix(err, "sync configmap", 0)
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
	if t.StorageProvider != nil {
		SetStorageProviderConfig(*t.StorageProvider, specRef)
	}
	SyncPodMeta(&t.TargetTemplate.ObjectMeta, t.PodSet)
	if t.MutatePod != nil {
		t.MutatePod(t.TargetTemplate)
	}
	if v1alpha1.GateInplaceConfigmapUpdate.Enabled(t.PodSet.GetOperatorVersion()) {
		t.TargetTemplate.ObjectMeta.Annotations[ConfigSuffixAnno] = t.ConfigSuffix
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
	if v1alpha1.GateInplaceConfigmapUpdate.Enabled(p.GetOperatorVersion()) {
		c.Env = append(c.Env, util.FieldRefEnv(ConfigSuffixEnvKey, fmt.Sprintf("metadata.annotations['%s']", ConfigSuffixAnno)))
	}
	memLimitEnv := GoMemLimitEnv(p.MemoryLimitPercent, c.Resources.Limits.Memory(), p.Overlay)
	if memLimitEnv != nil {
		c.Env = append(c.Env, *memLimitEnv)
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

func GoMemLimitEnv(memPercent *int, memoryLimit *resource.Quantity, overlay *v1alpha1.Overlay) *corev1.EnvVar {
	if memPercent == nil {
		return nil
	}
	if memoryLimit == nil || memoryLimit.Value() == 0 {
		return nil
	}

	// skip add GOMEMLIMIT env if it already added in overlay
	overLayEnv := util.FindFirst(overlay.Env, func(envVar corev1.EnvVar) bool {
		return envVar.Name == v1alpha1.EnvGoMemLimit
	})
	if overLayEnv != nil {
		return nil
	}

	memLimit := memoryLimit.Value() * int64(*memPercent) / 100
	envVar := &corev1.EnvVar{
		Name:  v1alpha1.EnvGoMemLimit,
		Value: strconv.Itoa(int(memLimit)),
	}
	return envVar
}
