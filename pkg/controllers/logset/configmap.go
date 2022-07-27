package logset

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConfigFile = "logservice.toml"
)

// buildConfigMap build the config map for log service
func buildConfigMap(ls *v1alpha1.LogSet) (*corev1.ConfigMap, error) {
	conf := ls.Spec.Config
	if conf == nil {
		conf = v1alpha1.NewTomlConfig(map[string]interface{}{})
	}
	// TODO(aylei): set HAKeeper initial configs according to the config schema of log service
	s, err := conf.ToString()
	if err != nil {
		return nil, err
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ls.Namespace,
			Name:      configMapName(ls),
			Labels:    common.SubResourceLabels(ls),
		},
		Data: map[string]string{
			ConfigFile: s,
		},
	}, nil
}

func configMapName(ls *v1alpha1.LogSet) string {
	return ls.Name + "-config"
}
