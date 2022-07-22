package logset

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildDiscoveryService(ls *v1alpha1.LogSet) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ls.Namespace,
			Name:      discoverySvcName(ls),
			Labels:    common.SubResourceLabels(ls),
		},
		// TODO(aylei): ports definition
		Spec: corev1.ServiceSpec{
			// TODO(aylei): determine haKeeper discovery service port
			// Ports: []corev1.ServicePort{}
			// service type might need to be configurable since the components
			// might not placed in a same k8s cluster
			Type:     corev1.ServiceTypeClusterIP,
			Selector: common.SubResourceLabels(ls),
		},
	}
}

func discoverySvcName(ls *v1alpha1.LogSet) string {
	return ls.Name + "-discovery"
}
