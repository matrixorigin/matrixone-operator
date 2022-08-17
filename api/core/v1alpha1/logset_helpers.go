package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

func (l *LogSet) StoresFailedFor(d time.Duration) []LogStore {
	var stores []LogStore
	for _, store := range l.Status.FailedStores {
		if time.Now().Sub(store.LastTransitionTime.Time) >= d {
			stores = append(stores, store)
		}
	}
	return stores
}

func (l *LogSet) AsDependency() LogSetRef {
	return LogSetRef{
		LogSet: &LogSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: l.Namespace,
				Name:      l.Name,
			},
		},
	}
}
