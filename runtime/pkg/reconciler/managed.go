// Copyright 2022 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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

func IsSynced(c Conditional) bool {
	cond, ok := GetCondition(c, ConditionTypeSynced)
	if ok {
		return cond.Status == metav1.ConditionTrue
	}
	return false
}
