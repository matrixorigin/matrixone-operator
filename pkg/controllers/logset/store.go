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
	var avalable []v1alpha1.LogStore
	for _, pod := range pods {
		store := v1alpha1.LogStore{
			PodName:            pod.Name,
			Phase:              v1alpha1.StorePhaseUp,
			LastTransitionTime: metav1.Time{time.Now()},
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
			avalable = append(avalable, store)
		} else {
			failed = append(avalable, store)
		}
	}
	ls.Status.FailedStores = failed
	ls.Status.AvailableStores = avalable
}
