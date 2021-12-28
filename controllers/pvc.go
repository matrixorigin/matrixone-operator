package controllers

import (
	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getPersistentVolumeClaim(moc *matrixonev1alpha1.MatrixoneCluster) []v1.PersistentVolumeClaim {
	pvc := []v1.PersistentVolumeClaim{}

	for _, val := range moc.Spec.VolumeClaimTemplates {
		pvc = append(pvc, val)
	}
	return pvc

}

func makePersistentVolumeClaim(pvc *v1.PersistentVolumeClaim, moc *matrixonev1alpha1.MatrixoneCluster, ls map[string]string, uniqueStr string) (*v1.PersistentVolumeClaim, error) {

	pvc.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "PersistentVolumeClaim",
	}

	pvc.ObjectMeta.Namespace = moc.Namespace

	if pvc.ObjectMeta.Labels == nil {
		pvc.ObjectMeta.Labels = ls
	} else {
		for k, v := range ls {
			pvc.ObjectMeta.Labels[k] = v
		}
	}

	if pvc.ObjectMeta.Name == "" {
		pvc.ObjectMeta.Name = uniqueStr
	}

	return pvc, nil
}
