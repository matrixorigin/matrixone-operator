package common

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	InstanceLabelKey  = "matrixorigin.io/instance"
	ComponentLabelKey = "matrixorigin.io/component"
	// NamespaceLabelKey is the label key for cluster-scope resources
	NamespaceLabelKey = "matrixorigin.io/namespace"
)

func SubResourceLabels(owner client.Object) map[string]string {
	return map[string]string{
		NamespaceLabelKey: owner.GetNamespace(),
		InstanceLabelKey:  owner.GetName(),
		ComponentLabelKey: owner.GetObjectKind().GroupVersionKind().Kind,
	}
}
