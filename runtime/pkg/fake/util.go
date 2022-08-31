package fake

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ReadyPod(meta metav1.ObjectMeta) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: meta,
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}},
		},
	}
}

func UnreadyPod(meta metav1.ObjectMeta) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: meta,
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionFalse,
			}},
		},
	}
}
