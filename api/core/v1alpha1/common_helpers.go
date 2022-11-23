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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

const (
	reasonEmpty = "empty"
)

func (c *ConditionalStatus) SetCondition(condition metav1.Condition) {
	if c.Conditions == nil {
		c.Conditions = []metav1.Condition{}
	}
	if condition.Reason == "" {
		condition.Reason = reasonEmpty
	}
	meta.SetStatusCondition(&c.Conditions, condition)
}

func (c *ConditionalStatus) GetConditions() []metav1.Condition {
	if c == nil {
		return nil
	}
	return c.Conditions
}

func (o *Overlay) OverlayPodMeta(meta *metav1.ObjectMeta) {
	if o == nil {
		return
	}
	if o.PodLabels != nil {
		// we are risking overwrite original labels here, this is desirable since overlay is
		// for advanced use-case and we should allow fine-grained (through risky) control
		for k, v := range o.PodLabels {
			meta.Labels[k] = v
		}
	}
	if o.PodAnnotations != nil {
		for k, v := range o.PodAnnotations {
			meta.Annotations[k] = v
		}
	}
}

// AppendVolumeClaims append the volume claims to the given claims
func (o *Overlay) AppendVolumeClaims(claims *[]corev1.PersistentVolumeClaim) {
	if o == nil {
		return
	}
	// TODO(aylei): maybe we need to append the overlay volume claims to the volume claim template
	*claims = append(*claims, o.VolumeClaims...)
}

func (o *Overlay) OverlayPodSpec(pod *corev1.PodSpec) {
	if o == nil {
		return
	}
	// TODO(aylei): append is not idempotent, should upsert instead
	//if o.Volumes != nil {
	//	pod.Volumes = o.Volumes
	//}
	if o.Affinity != nil {
		pod.Affinity = o.Affinity
	}
	if o.ServiceAccountName != "" {
		pod.ServiceAccountName = o.ServiceAccountName
	}
	if o.SecurityContext != nil {
		pod.SecurityContext = o.SecurityContext
	}
	if o.ImagePullSecrets != nil {
		pod.ImagePullSecrets = o.ImagePullSecrets
	}
	if o.Affinity != nil {
		pod.Affinity = o.Affinity
	}
	if o.Tolerations != nil {
		pod.Tolerations = o.Tolerations
	}
	if o.PriorityClassName != "" {
		pod.PriorityClassName = o.PriorityClassName
	}
	if o.TerminationGracePeriodSeconds != nil {
		pod.TerminationGracePeriodSeconds = o.TerminationGracePeriodSeconds
	}
	if o.HostAliases != nil {
		pod.HostAliases = o.HostAliases
	}
	if o.TopologySpreadConstraints != nil {
		// overwrite any pre-generated topologySpreadConstraints if an overlay is set
		pod.TopologySpreadConstraints = o.TopologySpreadConstraints
	}
	if o.RuntimeClassName != nil {
		pod.RuntimeClassName = o.RuntimeClassName
	}
	if o.DNSConfig != nil {
		pod.DNSConfig = o.DNSConfig
	}
	if o.InitContainers != nil {
		// overwrite init containers if an overlay is set
		pod.InitContainers = o.InitContainers
	}
	if o.SidecarContainers != nil {
		// overwrite all containers except "main" if an overlay is set
		var containers []corev1.Container
		main := findMainContainer(pod.Containers)
		if main != nil {
			containers = append(containers, *main)
		}
		containers = append(containers, o.SidecarContainers...)
		pod.Containers = containers
	}
}

func (o *Overlay) OverlayMainContainer(c *corev1.Container) {
	if o == nil {
		return
	}
	mc := o.MainContainerOverlay
	if mc.ImagePullPolicy != nil {
		c.ImagePullPolicy = *o.ImagePullPolicy
	}
	if mc.Command != nil {
		c.Command = mc.Command
	}
	if mc.Args != nil {
		c.Args = mc.Args
	}
	if mc.EnvFrom != nil {
		c.EnvFrom = mc.EnvFrom
	}
	if mc.Env != nil {
		c.Env = mc.Env
	}
	if mc.ReadinessProbe != nil {
		c.ReadinessProbe = mc.ReadinessProbe
	}
	if mc.LivenessProbe != nil {
		c.LivenessProbe = mc.LivenessProbe
	}
	if mc.Lifecycle != nil {
		c.Lifecycle = mc.Lifecycle
	}
	// TODO(aylei): append is not idempotent, should upsert instead
	//if mc.VolumeMounts != nil {
	//	c.VolumeMounts = o.VolumeMounts
	//}
}

func (s *FailoverStatus) StoresFailedFor(d time.Duration) []Store {
	var stores []Store

	for _, store := range s.FailedStores {
		if time.Now().Sub(store.LastTransitionTime.Time) >= d {
			stores = append(stores, store)
		}
	}

	return stores
}

func findMainContainer(containers []corev1.Container) *corev1.Container {
	for _, c := range containers {
		if c.Name == ContainerMain {
			return &c
		}
	}
	return nil
}
