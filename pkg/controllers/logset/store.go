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
package logset

import (
	"time"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func collectStoreStatus(ls *v1alpha1.LogSet, pods []corev1.Pod) {
	previousStore := map[string]v1alpha1.LogStore{}
	for _, store := range ls.Status.FailedStores {
		previousStore[store.PodName] = store
	}
	for _, store := range ls.Status.AvailableStores {
		previousStore[store.PodName] = store
	}
	var failed []v1alpha1.LogStore
	var available []v1alpha1.LogStore
	for _, pod := range pods {
		store := v1alpha1.LogStore{
			PodName:            pod.Name,
			Phase:              v1alpha1.StorePhaseUp,
			LastTransitionTime: metav1.Time{Time: time.Now()},
		}
		if !util.IsPodReady(&pod) {
			store.Phase = v1alpha1.StorePhaseDown
		}
		// update last transition time
		if previous, ok := previousStore[store.PodName]; ok {
			if previous.Phase == store.Phase {
				// phase not changed, keep last transition time
				store.LastTransitionTime = previous.LastTransitionTime
			}
		}
		if store.Phase == v1alpha1.StorePhaseUp {
			available = append(available, store)
		} else {
			failed = append(failed, store)
		}
	}
	ls.Status.FailedStores = failed
	ls.Status.AvailableStores = available
}
