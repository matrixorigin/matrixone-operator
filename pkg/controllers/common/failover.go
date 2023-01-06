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
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

const (
	// TODO: maybe we need to make this configurable
	minReadySeconds = 15
)

type StoreFn func(store *v1alpha1.Store)

// CollectStoreStatus is a template method to collect store status.
// fns allows the caller to pass a list of functions set the store status according to other information (e.g. query HA Keeper)
func CollectStoreStatus(status *v1alpha1.FailoverStatus, pods []corev1.Pod, fns ...StoreFn) {
	previousStore := map[string]v1alpha1.Store{}
	for _, store := range status.FailedStores {
		previousStore[store.PodName] = store
	}
	for _, store := range status.AvailableStores {
		previousStore[store.PodName] = store
	}
	var availableStores []v1alpha1.Store
	var failedStores []v1alpha1.Store
	for _, pod := range pods {
		store := v1alpha1.Store{
			PodName:            pod.Name,
			Phase:              v1alpha1.StorePhaseUp,
			LastTransitionTime: metav1.Time{Time: time.Now()},
		}
		if !util.IsPodAvailable(&pod, minReadySeconds, metav1.Time{Time: time.Now()}) {
			store.Phase = v1alpha1.StorePhaseDown
		}
		for _, fn := range fns {
			fn(&store)
		}
		// update last transition time
		if previous, ok := previousStore[store.PodName]; ok {
			if previous.Phase == store.Phase {
				// phase not changed, keep last transition time
				store.LastTransitionTime = previous.LastTransitionTime
			}
		}
		if store.Phase == v1alpha1.StorePhaseUp {
			availableStores = append(availableStores, store)
		} else {
			failedStores = append(failedStores, store)
		}
	}
	status.AvailableStores = availableStores
	status.FailedStores = failedStores
	return
}
