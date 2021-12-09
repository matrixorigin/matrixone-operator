package k8sutils

import (
	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func generateMetaInformation(resourceKind string, apiVersion string) metav1.TypeMeta {
	return metav1.TypeMeta{
		Kind:       resourceKind,
		APIVersion: apiVersion,
	}
}

func generateObjectMetaInformation(name string, namespace string, labels map[string]string, annotations map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   namespace,
		Labels:      labels,
		Annotations: annotations,
	}
}

func AddOwnerRefToObject(obj metav1.Object, ownerRef metav1.OwnerReference) {
	obj.SetOwnerReferences(append(obj.GetOwnerReferences(), ownerRef))
}

func AsOwner(cr *matrixonev1alpha1.Matrixone) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: cr.APIVersion,
		Kind:       cr.Kind,
		Name:       cr.Name,
		UID:        cr.UID,
		Controller: &trueVar,
	}
}

func matrixoneAsOwner(cr *matrixonev1alpha1.Matrixone) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: cr.APIVersion,
		Kind:       cr.Kind,
		Name:       cr.Name,
		UID:        cr.UID,
		Controller: &trueVar,
	}
}

func generateStatefulSetsAnots() map[string]string {
	return map[string]string{
		"matrxonne.labs.in":    "true",
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "9121",
	}
}

func generateServiceAnots() map[string]string {
	return map[string]string{
		"matrxone.labs.in":     "true",
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "9121",
	}
}

func LabelSelectors(labels map[string]string) *metav1.LabelSelector {
	return &metav1.LabelSelector{MatchLabels: labels}
}
