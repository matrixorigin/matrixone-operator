// Copyright 2024 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	CNStateAnno = "matrixorigin.io/cn-state"

	CNDrainingFinalizer = "matrixorigin.io/cn-draining"

	CNStoreReadiness corev1.PodConditionType = "matrixorigin.io/cn-store"
)

func AddReadinessGate(podSpec *corev1.PodSpec, ct corev1.PodConditionType) {
	for _, r := range podSpec.ReadinessGates {
		if r.ConditionType == ct {
			return
		}
	}
	podSpec.ReadinessGates = append(podSpec.ReadinessGates, corev1.PodReadinessGate{
		ConditionType: ct,
	})
}

func GetReadinessCondition(pod *corev1.Pod, conditionType corev1.PodConditionType) *corev1.PodCondition {
	if pod == nil {
		return nil
	}
	for i := range pod.Status.Conditions {
		c := &pod.Status.Conditions[i]
		if c.Type == conditionType {
			return c
		}
	}
	return nil
}

func NewCNReadinessCondition(status corev1.ConditionStatus, msg string) corev1.PodCondition {
	return corev1.PodCondition{
		Type:               CNStoreReadiness,
		Message:            msg,
		Status:             status,
		LastTransitionTime: metav1.Now(),
	}
}
