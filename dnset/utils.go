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

package dnset

import (
	"time"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
)

// collectDNStoreStatus: record dn pod status
func collectDNStoreStatus(obj *v1alpha1.CNSet, pods []corev1.Pod) {
	previousStore := map[string]v1alpha1.DNStore{}
	for _, store := range obj.Status.FailedStores {
		previousStore[store.PodName] = store
	}

	for _, store := range obj.Status.AvailableStores {
		previousStore[store.PodName] = store
	}

	var failed []v1alpha1.DNStore
	var avalable []v1alpha1.DNStore
	for _, pod := range pods {
		store := v1alpha1.DNStore{
			PodName:            pod.Name,
			LastTransitionTime: metav1.Time{time.Now{}},
		}

		if util.IsPodReay(&pod) {
			store.Phase = v1alpha1.StorePhaseUp
		} else {
			store.Phase = v1alpha1.StorePhaseDown
		}

		if previous, ok := previousStore[store.PodName]; ok {
			if previous.Phase == store.Phase {
				store.LastTransitionTime = previous.LastTransitionTime
			}
		}
		if store.Phase == v1alpha1.StorePhaseUp {
			avalable = append(avalable, store)
		} else {
			failed = append(avalable, store)
		}

	}

	obj.Status.FailedStore = failed
	obj.Status.AvailableStores = available
}
