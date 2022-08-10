package v1alpha1

import "fmt"

func (l *LogSet) HAKeeperClientConfig() *TomlConfig {
	if l.Status.Discovery == nil {
		return nil
	}
	tc := NewTomlConfig(map[string]interface{}{})
	tc.Set([]string{"hakeeper-client", "discovery-address"}, fmt.Sprintf("%s:%d", l.Status.Discovery.Address, l.Status.Discovery.Port))
	return tc
}
