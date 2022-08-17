package reconciler

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConditionType string

const (
	// ConditionTypeReady Whether the object is ready to act
	ConditionTypeReady = "Ready"
	// ConditionTypeSynced Whether the object is update to date
	ConditionTypeSynced = "Synced"
)

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

type Conditional interface {
	SetCondition(c metav1.Condition)
	GetConditions() []metav1.Condition
}

func GetCondition(c Conditional, conditionType ConditionType) (*metav1.Condition, bool) {
	cs := c.GetConditions()
	for i := range cs {
		if cs[i].Type == string(conditionType) {
			return &cs[i], true
		}
	}
	return nil, false
}

func IsReady(c Conditional) bool {
	cond, ok := GetCondition(c, ConditionTypeReady)
	if ok {
		return cond.Status == metav1.ConditionTrue
	}
	return false
}
