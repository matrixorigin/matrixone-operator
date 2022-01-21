package components

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MakePersistentVolumeClaim() error {
	return nil
}

func MakePersistentVolumeClaimListEmptyObj() *corev1.PersistentVolumeClaimList {
	return &corev1.PersistentVolumeClaimList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
	}
}
