package controllers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func labelsForMatrixone(name string) map[string]string {
	return map[string]string{"service": "app", "cr": name}
}

func labelSelector(labels map[string]string) *metav1.LabelSelector {
	return &metav1.LabelSelector{MatchLabels: labels}
}
