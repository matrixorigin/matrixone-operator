package controllers

import (
	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makePodTemplate(moc *matrixonev1alpha1.MatrixoneCluster, ls map[string]string, uniqueStr string) v1.PodTemplateSpec {
	return v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: ls,
		},
		Spec: makePodSpec(moc, uniqueStr),
	}
}

func makePodSpec(moc *matrixonev1alpha1.MatrixoneCluster, uniqueStr string) v1.PodSpec {
	spec := v1.PodSpec{
		NodeSelector: moc.Spec.NodeSelector,
		Tolerations:  getTolerations(moc),
		Affinity:     getAffinity(moc),
		Containers: []v1.Container{
			{},
		},
		TerminationGracePeriodSeconds: moc.Spec.TerminationGracePeriodSeconds,
		ServiceAccountName:            moc.Spec.ServiceAccount,
	}
	return spec
}

func getTolerations(moc *matrixonev1alpha1.MatrixoneCluster) []v1.Toleration {
	tolerations := []v1.Toleration{}

	for _, val := range moc.Spec.Tolerations {
		tolerations = append(tolerations, val)
	}

	return tolerations
}

func getAffinity(moc *matrixonev1alpha1.MatrixoneCluster) *v1.Affinity {
	affinity := firstNonNilValue(moc.Spec.Affinity, &v1.Affinity{}).(*v1.Affinity)
	return affinity
}
