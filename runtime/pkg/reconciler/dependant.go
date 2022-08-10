package reconciler

import "sigs.k8s.io/controller-runtime/pkg/client"

type Dependant interface {
	GetDependencies() []Dependency
}

type Dependency interface {
	// IsReady checks whether the given object is ready
	IsReady(kubeCli KubeClient) (bool, error)
}

type ObjectDependency[T client.Object] struct {
	ObjectRef T
	ReadyFunc func(T) bool
}

func (od *ObjectDependency[T]) IsReady(kubeCli KubeClient) (bool, error) {
	// 1. refresh the status of the dependency
	obj := od.ObjectRef
	err := kubeCli.Get(client.ObjectKeyFromObject(obj), obj)
	if err != nil {
		return false, err
	}
	return od.ReadyFunc(obj), nil
}
