// Copyright 2025-2026 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

package cnstore

import (
	"testing"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPythonUDFEnabledSurvivesMarkerRemoval(t *testing.T) {
	c := &withCNSet{}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
		v1alpha1.UDFWorkerEnabledLabel: v1alpha1.UDFWorkerEnabledValue,
	}}}
	if !c.pythonUDFEnabled(pod) {
		t.Fatal("worker marker should identify a Python-enabled Pod")
	}

	delete(pod.Labels, v1alpha1.UDFWorkerEnabledLabel)
	pod.Spec.Containers = []corev1.Container{{Name: v1alpha1.ContainerUDFWorker}}
	if !c.pythonUDFEnabled(pod) {
		t.Fatal("Worker container should identify a Python-enabled Pod after marker removal")
	}

	pod.Spec.Containers = nil
	c.cn = &v1alpha1.CNSet{Spec: v1alpha1.CNSetSpec{
		ConfigThatChangeCNSpec: v1alpha1.ConfigThatChangeCNSpec{
			UDFWorker: &v1alpha1.UDFWorkerPolicy{Enabled: true},
		},
	}}
	if !c.pythonUDFEnabled(pod) {
		t.Fatal("effective CNSet policy should identify a Python-enabled Pod")
	}
}
