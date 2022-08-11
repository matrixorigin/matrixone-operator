package v1alpha1

import (
	"fmt"
	"time"
)

func (l *LogSet) HAKeeperClientConfig() *TomlConfig {
	if l.Status.Discovery == nil {
		return nil
	}
	tc := NewTomlConfig(map[string]interface{}{})
	tc.Set([]string{"hakeeper-client", "discovery-address"}, fmt.Sprintf("%s:%d", l.Status.Discovery.Address, l.Status.Discovery.Port))
	return tc
}

func (l *LogSet) StoresFailedFor(d time.Duration) []LogStore {
	var stores []LogStore
	for _, store := range l.Status.FailedStores {
		if time.Now().Sub(store.LastTransitionTime.Time) >= d {
			stores = append(stores, store)
		}
	}
	return stores
}
