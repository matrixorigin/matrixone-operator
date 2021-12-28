package controllers

import (
	"fmt"

	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeStatefulSetListEmptyObj() *appsv1.StatefulSetList {
	return &appsv1.StatefulSetList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
	}
}

func makeStatefulSet(moc *matrixonev1alpha1.MatrixoneCluster, ls map[string]string, serviceName string, uniqueStr string) (*appsv1.StatefulSet, error) {
	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s", uniqueStr),
			Namespace: moc.Namespace,
			Labels:    ls,
		},
		Spec: makeStatefulSetSpec(moc, ls, serviceName, uniqueStr),
	}, nil
}

func makeStatefulSetSpec(moc *matrixonev1alpha1.MatrixoneCluster, ls map[string]string, serviceName, uniqueStr string) appsv1.StatefulSetSpec {
	updateStrategy := firstNonNilValue(moc.Spec.UpdateStrategy, &appsv1.StatefulSetUpdateStrategy{}).(*appsv1.StatefulSetUpdateStrategy)

	stsSpec := appsv1.StatefulSetSpec{
		ServiceName: serviceName,
		Selector: &metav1.LabelSelector{
			MatchLabels: ls,
		},
		Replicas:             &moc.Spec.Replicas,
		UpdateStrategy:       *updateStrategy,
		Template:             makePodTemplate(moc, ls, uniqueStr),
		VolumeClaimTemplates: getPersistentVolumeClaim(moc),
	}

	return stsSpec
}
