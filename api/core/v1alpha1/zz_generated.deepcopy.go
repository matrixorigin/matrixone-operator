//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CNSet) DeepCopyInto(out *CNSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Deps.DeepCopyInto(&out.Deps)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CNSet.
func (in *CNSet) DeepCopy() *CNSet {
	if in == nil {
		return nil
	}
	out := new(CNSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CNSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CNSetBasic) DeepCopyInto(out *CNSetBasic) {
	*out = *in
	in.PodSet.DeepCopyInto(&out.PodSet)
	if in.CacheVolume != nil {
		in, out := &in.CacheVolume, &out.CacheVolume
		*out = new(Volume)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CNSetBasic.
func (in *CNSetBasic) DeepCopy() *CNSetBasic {
	if in == nil {
		return nil
	}
	out := new(CNSetBasic)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CNSetDeps) DeepCopyInto(out *CNSetDeps) {
	*out = *in
	in.LogSetRef.DeepCopyInto(&out.LogSetRef)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CNSetDeps.
func (in *CNSetDeps) DeepCopy() *CNSetDeps {
	if in == nil {
		return nil
	}
	out := new(CNSetDeps)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CNSetList) DeepCopyInto(out *CNSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CNSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CNSetList.
func (in *CNSetList) DeepCopy() *CNSetList {
	if in == nil {
		return nil
	}
	out := new(CNSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CNSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CNSetSpec) DeepCopyInto(out *CNSetSpec) {
	*out = *in
	in.CNSetBasic.DeepCopyInto(&out.CNSetBasic)
	if in.Overlay != nil {
		in, out := &in.Overlay, &out.Overlay
		*out = new(Overlay)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CNSetSpec.
func (in *CNSetSpec) DeepCopy() *CNSetSpec {
	if in == nil {
		return nil
	}
	out := new(CNSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CNSetStatus) DeepCopyInto(out *CNSetStatus) {
	*out = *in
	in.ConditionalStatus.DeepCopyInto(&out.ConditionalStatus)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CNSetStatus.
func (in *CNSetStatus) DeepCopy() *CNSetStatus {
	if in == nil {
		return nil
	}
	out := new(CNSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConditionalStatus) DeepCopyInto(out *ConditionalStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConditionalStatus.
func (in *ConditionalStatus) DeepCopy() *ConditionalStatus {
	if in == nil {
		return nil
	}
	out := new(ConditionalStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DNSet) DeepCopyInto(out *DNSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Deps.DeepCopyInto(&out.Deps)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DNSet.
func (in *DNSet) DeepCopy() *DNSet {
	if in == nil {
		return nil
	}
	out := new(DNSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DNSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DNSetBasic) DeepCopyInto(out *DNSetBasic) {
	*out = *in
	in.PodSet.DeepCopyInto(&out.PodSet)
	if in.CacheVolume != nil {
		in, out := &in.CacheVolume, &out.CacheVolume
		*out = new(Volume)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DNSetBasic.
func (in *DNSetBasic) DeepCopy() *DNSetBasic {
	if in == nil {
		return nil
	}
	out := new(DNSetBasic)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DNSetDeps) DeepCopyInto(out *DNSetDeps) {
	*out = *in
	in.LogSetRef.DeepCopyInto(&out.LogSetRef)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DNSetDeps.
func (in *DNSetDeps) DeepCopy() *DNSetDeps {
	if in == nil {
		return nil
	}
	out := new(DNSetDeps)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DNSetList) DeepCopyInto(out *DNSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DNSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DNSetList.
func (in *DNSetList) DeepCopy() *DNSetList {
	if in == nil {
		return nil
	}
	out := new(DNSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DNSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DNSetSpec) DeepCopyInto(out *DNSetSpec) {
	*out = *in
	in.DNSetBasic.DeepCopyInto(&out.DNSetBasic)
	if in.Overlay != nil {
		in, out := &in.Overlay, &out.Overlay
		*out = new(Overlay)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DNSetSpec.
func (in *DNSetSpec) DeepCopy() *DNSetSpec {
	if in == nil {
		return nil
	}
	out := new(DNSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DNSetStatus) DeepCopyInto(out *DNSetStatus) {
	*out = *in
	in.ConditionalStatus.DeepCopyInto(&out.ConditionalStatus)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DNSetStatus.
func (in *DNSetStatus) DeepCopy() *DNSetStatus {
	if in == nil {
		return nil
	}
	out := new(DNSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExternalLogSet) DeepCopyInto(out *ExternalLogSet) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExternalLogSet.
func (in *ExternalLogSet) DeepCopy() *ExternalLogSet {
	if in == nil {
		return nil
	}
	out := new(ExternalLogSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InitialConfig) DeepCopyInto(out *InitialConfig) {
	*out = *in
	if in.LogShards != nil {
		in, out := &in.LogShards, &out.LogShards
		*out = new(int)
		**out = **in
	}
	if in.DNShards != nil {
		in, out := &in.DNShards, &out.DNShards
		*out = new(int)
		**out = **in
	}
	if in.HAKeeperReplicas != nil {
		in, out := &in.HAKeeperReplicas, &out.HAKeeperReplicas
		*out = new(int)
		**out = **in
	}
	if in.LogShardReplicas != nil {
		in, out := &in.LogShardReplicas, &out.LogShardReplicas
		*out = new(int)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InitialConfig.
func (in *InitialConfig) DeepCopy() *InitialConfig {
	if in == nil {
		return nil
	}
	out := new(InitialConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogSet) DeepCopyInto(out *LogSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogSet.
func (in *LogSet) DeepCopy() *LogSet {
	if in == nil {
		return nil
	}
	out := new(LogSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LogSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogSetBasic) DeepCopyInto(out *LogSetBasic) {
	*out = *in
	in.PodSet.DeepCopyInto(&out.PodSet)
	in.Volume.DeepCopyInto(&out.Volume)
	in.SharedStorage.DeepCopyInto(&out.SharedStorage)
	in.InitialConfig.DeepCopyInto(&out.InitialConfig)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogSetBasic.
func (in *LogSetBasic) DeepCopy() *LogSetBasic {
	if in == nil {
		return nil
	}
	out := new(LogSetBasic)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogSetDiscovery) DeepCopyInto(out *LogSetDiscovery) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogSetDiscovery.
func (in *LogSetDiscovery) DeepCopy() *LogSetDiscovery {
	if in == nil {
		return nil
	}
	out := new(LogSetDiscovery)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogSetList) DeepCopyInto(out *LogSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LogSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogSetList.
func (in *LogSetList) DeepCopy() *LogSetList {
	if in == nil {
		return nil
	}
	out := new(LogSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LogSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogSetRef) DeepCopyInto(out *LogSetRef) {
	*out = *in
	if in.LogSet != nil {
		in, out := &in.LogSet, &out.LogSet
		*out = new(LogSet)
		(*in).DeepCopyInto(*out)
	}
	if in.ExternalLogSet != nil {
		in, out := &in.ExternalLogSet, &out.ExternalLogSet
		*out = new(ExternalLogSet)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogSetRef.
func (in *LogSetRef) DeepCopy() *LogSetRef {
	if in == nil {
		return nil
	}
	out := new(LogSetRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogSetSpec) DeepCopyInto(out *LogSetSpec) {
	*out = *in
	in.LogSetBasic.DeepCopyInto(&out.LogSetBasic)
	if in.Overlay != nil {
		in, out := &in.Overlay, &out.Overlay
		*out = new(Overlay)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogSetSpec.
func (in *LogSetSpec) DeepCopy() *LogSetSpec {
	if in == nil {
		return nil
	}
	out := new(LogSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogSetStatus) DeepCopyInto(out *LogSetStatus) {
	*out = *in
	in.ConditionalStatus.DeepCopyInto(&out.ConditionalStatus)
	if in.AvailableStores != nil {
		in, out := &in.AvailableStores, &out.AvailableStores
		*out = make([]LogStore, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.FailedStores != nil {
		in, out := &in.FailedStores, &out.FailedStores
		*out = make([]LogStore, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Discovery != nil {
		in, out := &in.Discovery, &out.Discovery
		*out = new(LogSetDiscovery)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogSetStatus.
func (in *LogSetStatus) DeepCopy() *LogSetStatus {
	if in == nil {
		return nil
	}
	out := new(LogSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LogStore) DeepCopyInto(out *LogStore) {
	*out = *in
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LogStore.
func (in *LogStore) DeepCopy() *LogStore {
	if in == nil {
		return nil
	}
	out := new(LogStore)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MainContainer) DeepCopyInto(out *MainContainer) {
	*out = *in
	in.Resources.DeepCopyInto(&out.Resources)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MainContainer.
func (in *MainContainer) DeepCopy() *MainContainer {
	if in == nil {
		return nil
	}
	out := new(MainContainer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MainContainerOverlay) DeepCopyInto(out *MainContainerOverlay) {
	*out = *in
	if in.Command != nil {
		in, out := &in.Command, &out.Command
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.EnvFrom != nil {
		in, out := &in.EnvFrom, &out.EnvFrom
		*out = make([]corev1.EnvFromSource, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]corev1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.VolumeMounts != nil {
		in, out := &in.VolumeMounts, &out.VolumeMounts
		*out = make([]corev1.VolumeMount, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.LivenessProbe != nil {
		in, out := &in.LivenessProbe, &out.LivenessProbe
		*out = new(corev1.Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.ReadinessProbe != nil {
		in, out := &in.ReadinessProbe, &out.ReadinessProbe
		*out = new(corev1.Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.Lifecycle != nil {
		in, out := &in.Lifecycle, &out.Lifecycle
		*out = new(corev1.Lifecycle)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MainContainerOverlay.
func (in *MainContainerOverlay) DeepCopy() *MainContainerOverlay {
	if in == nil {
		return nil
	}
	out := new(MainContainerOverlay)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MatrixOneCluster) DeepCopyInto(out *MatrixOneCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MatrixOneCluster.
func (in *MatrixOneCluster) DeepCopy() *MatrixOneCluster {
	if in == nil {
		return nil
	}
	out := new(MatrixOneCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MatrixOneCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MatrixOneClusterList) DeepCopyInto(out *MatrixOneClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MatrixOneCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MatrixOneClusterList.
func (in *MatrixOneClusterList) DeepCopy() *MatrixOneClusterList {
	if in == nil {
		return nil
	}
	out := new(MatrixOneClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MatrixOneClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MatrixOneClusterSpec) DeepCopyInto(out *MatrixOneClusterSpec) {
	*out = *in
	in.TP.DeepCopyInto(&out.TP)
	if in.AP != nil {
		in, out := &in.AP, &out.AP
		*out = new(CNSetBasic)
		(*in).DeepCopyInto(*out)
	}
	in.DN.DeepCopyInto(&out.DN)
	in.LogService.DeepCopyInto(&out.LogService)
	if in.WebUIEnabled != nil {
		in, out := &in.WebUIEnabled, &out.WebUIEnabled
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MatrixOneClusterSpec.
func (in *MatrixOneClusterSpec) DeepCopy() *MatrixOneClusterSpec {
	if in == nil {
		return nil
	}
	out := new(MatrixOneClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MatrixOneClusterStatus) DeepCopyInto(out *MatrixOneClusterStatus) {
	*out = *in
	in.ConditionalStatus.DeepCopyInto(&out.ConditionalStatus)
	if in.TP != nil {
		in, out := &in.TP, &out.TP
		*out = new(CNSetStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.AP != nil {
		in, out := &in.AP, &out.AP
		*out = new(CNSetStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.DN != nil {
		in, out := &in.DN, &out.DN
		*out = new(DNSetStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.LogService != nil {
		in, out := &in.LogService, &out.LogService
		*out = new(LogSetStatus)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MatrixOneClusterStatus.
func (in *MatrixOneClusterStatus) DeepCopy() *MatrixOneClusterStatus {
	if in == nil {
		return nil
	}
	out := new(MatrixOneClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Overlay) DeepCopyInto(out *Overlay) {
	*out = *in
	in.MainContainerOverlay.DeepCopyInto(&out.MainContainerOverlay)
	if in.Volumes != nil {
		in, out := &in.Volumes, &out.Volumes
		*out = make([]corev1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.VolumeClaims != nil {
		in, out := &in.VolumeClaims, &out.VolumeClaims
		*out = make([]corev1.PersistentVolumeClaim, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.InitContainers != nil {
		in, out := &in.InitContainers, &out.InitContainers
		*out = make([]corev1.Container, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.SidecarContainers != nil {
		in, out := &in.SidecarContainers, &out.SidecarContainers
		*out = make([]corev1.Container, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.SecurityContext != nil {
		in, out := &in.SecurityContext, &out.SecurityContext
		*out = new(corev1.PodSecurityContext)
		(*in).DeepCopyInto(*out)
	}
	if in.ImagePullSecrets != nil {
		in, out := &in.ImagePullSecrets, &out.ImagePullSecrets
		*out = make([]corev1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(corev1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]corev1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.TerminationGracePeriodSeconds != nil {
		in, out := &in.TerminationGracePeriodSeconds, &out.TerminationGracePeriodSeconds
		*out = new(int64)
		**out = **in
	}
	if in.HostAliases != nil {
		in, out := &in.HostAliases, &out.HostAliases
		*out = make([]corev1.HostAlias, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.TopologySpreadConstraints != nil {
		in, out := &in.TopologySpreadConstraints, &out.TopologySpreadConstraints
		*out = make([]corev1.TopologySpreadConstraint, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.RuntimeClassName != nil {
		in, out := &in.RuntimeClassName, &out.RuntimeClassName
		*out = new(string)
		**out = **in
	}
	if in.DNSConfig != nil {
		in, out := &in.DNSConfig, &out.DNSConfig
		*out = new(corev1.PodDNSConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.PodLabels != nil {
		in, out := &in.PodLabels, &out.PodLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.PodAnnotations != nil {
		in, out := &in.PodAnnotations, &out.PodAnnotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Overlay.
func (in *Overlay) DeepCopy() *Overlay {
	if in == nil {
		return nil
	}
	out := new(Overlay)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodSet) DeepCopyInto(out *PodSet) {
	*out = *in
	in.MainContainer.DeepCopyInto(&out.MainContainer)
	if in.TopologyEvenSpread != nil {
		in, out := &in.TopologyEvenSpread, &out.TopologyEvenSpread
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Config != nil {
		in, out := &in.Config, &out.Config
		*out = (*in).DeepCopy()
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodSet.
func (in *PodSet) DeepCopy() *PodSet {
	if in == nil {
		return nil
	}
	out := new(PodSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *S3Provider) DeepCopyInto(out *S3Provider) {
	*out = *in
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(corev1.LocalObjectReference)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new S3Provider.
func (in *S3Provider) DeepCopy() *S3Provider {
	if in == nil {
		return nil
	}
	out := new(S3Provider)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SharedStorageProvider) DeepCopyInto(out *SharedStorageProvider) {
	*out = *in
	if in.S3 != nil {
		in, out := &in.S3, &out.S3
		*out = new(S3Provider)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SharedStorageProvider.
func (in *SharedStorageProvider) DeepCopy() *SharedStorageProvider {
	if in == nil {
		return nil
	}
	out := new(SharedStorageProvider)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Volume) DeepCopyInto(out *Volume) {
	*out = *in
	out.Size = in.Size.DeepCopy()
	if in.StorageClassName != nil {
		in, out := &in.StorageClassName, &out.StorageClassName
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Volume.
func (in *Volume) DeepCopy() *Volume {
	if in == nil {
		return nil
	}
	out := new(Volume)
	in.DeepCopyInto(out)
	return out
}





