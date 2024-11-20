// Copyright 2024 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"fmt"

	"github.com/go-errors/errors"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SyncCloneSetVolumeSize(kubeCli recon.KubeClient, owner client.Object, size resource.Quantity, cs *kruisev1alpha1.CloneSet) error {
	var changed bool
	for i := range cs.Spec.VolumeClaimTemplates {
		if cs.Spec.VolumeClaimTemplates[i].Name == DataVolume {
			oldSize := cs.Spec.VolumeClaimTemplates[i].Spec.Resources.Requests[corev1.ResourceStorage]
			c := oldSize.Cmp(size)
			if c < 0 {
				changed = true
				cs.Spec.VolumeClaimTemplates[i].Spec.Resources.Requests[corev1.ResourceStorage] = size
			} else if c > 0 {
				return errors.New(fmt.Sprintf("volume size cannot be decreased from %s to %s", oldSize.String(), size.String()))
			}
		}
	}
	if !changed {
		return nil
	}
	podList := &corev1.PodList{}
	err := kubeCli.List(podList, client.InNamespace(owner.GetNamespace()), client.MatchingLabels(SubResourceLabels(owner)))
	if err != nil {
		return errors.WrapPrefix(err, "list pods", 0)
	}
	for i := range podList.Items {
		pod := &podList.Items[i]
		instanceId := pod.Labels[kruisev1alpha1.CloneSetInstanceID]
		if instanceId == "" {
			continue
		}
		pvcList := &corev1.PersistentVolumeClaimList{}
		err := kubeCli.List(pvcList, client.InNamespace(owner.GetNamespace()), client.MatchingLabels(map[string]string{
			kruisev1alpha1.CloneSetInstanceID: instanceId,
		}))

		klog.Infof("sync volume size for %s, pvc list: %v", owner.GetName(), pvcList.Items)
		if err != nil {
			return errors.WrapPrefix(err, "list volumes", 0)
		}
		for j := range pvcList.Items {
			pvc := &pvcList.Items[j]
			current := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
			if current.Cmp(size) < 0 {
				if err := kubeCli.Patch(pvc, func() error {
					pvc.Spec.Resources.Requests[corev1.ResourceStorage] = size
					return nil
				}); err != nil {
					return errors.WrapPrefix(err, "patch volume size", 0)
				}
			}
		}
	}
	if err := kubeCli.Update(cs); err != nil {
		return errors.WrapPrefix(err, "sync volume size", 0)
	}
	return nil
}

// SyncStsVolumeSize syncs the volume size of component backed by kruise statefuset
func SyncStsVolumeSize(kubeCli recon.KubeClient, owner client.Object, size resource.Quantity, sts *kruisev1.StatefulSet) error {
	var changed bool
	for i := range sts.Spec.VolumeClaimTemplates {
		if sts.Spec.VolumeClaimTemplates[i].Name == DataVolume {
			oldSize := sts.Spec.VolumeClaimTemplates[i].Spec.Resources.Requests[corev1.ResourceStorage]
			c := oldSize.Cmp(size)
			if c < 0 {
				changed = true
				sts.Spec.VolumeClaimTemplates[i].Spec.Resources.Requests[corev1.ResourceStorage] = size
			} else if c > 0 {
				return errors.New(fmt.Sprintf("volume size cannot be decreased from %s to %s", oldSize.String(), size.String()))
			}
		}
	}
	if !changed {
		return nil
	}
	pvcList := &corev1.PersistentVolumeClaimList{}
	err := kubeCli.List(pvcList, client.InNamespace(owner.GetNamespace()), client.MatchingLabels(SubResourceLabels(owner)))
	if err != nil {
		return errors.WrapPrefix(err, "list volumes", 0)
	}
	for i := range pvcList.Items {
		pvc := &pvcList.Items[i]
		current := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		if current.Cmp(size) < 0 {
			if err := kubeCli.Patch(pvc, func() error {
				pvc.Spec.Resources.Requests[corev1.ResourceStorage] = size
				return nil
			}); err != nil {
				return errors.WrapPrefix(err, "patch volume size", 0)
			}
		}
	}
	if err := kubeCli.Update(sts); err != nil {
		return errors.WrapPrefix(err, "sync volume size", 0)
	}
	return nil
}
