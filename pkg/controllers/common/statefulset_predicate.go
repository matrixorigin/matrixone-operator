// Copyright 2026 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/openkruise/kruise-api/apps/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// LogSetStatefulSetOwner returns the LogSet controller key for a Kruise
// StatefulSet. It rejects similarly named resources owned by another API.
func LogSetStatefulSetOwner(object client.Object) (client.ObjectKey, bool) {
	sts, ok := object.(*v1beta1.StatefulSet)
	if !ok {
		return client.ObjectKey{}, false
	}
	owner := sts.GetOwnerReferences()
	for i := range owner {
		if owner[i].Controller != nil && *owner[i].Controller &&
			owner[i].APIVersion == v1alpha1.GroupVersion.String() && owner[i].Kind == "LogSet" {
			return client.ObjectKey{Namespace: sts.Namespace, Name: owner[i].Name}, true
		}
	}
	return client.ObjectKey{}, false
}

// ReferencesLogSet reports whether an internal LogSet reference points to key.
// Older embedded references may omit namespace; those are local to the
// dependent object for compatibility with the established API semantics.
func ReferencesLogSet(ref v1alpha1.LogSetRef, dependentNamespace string, key client.ObjectKey) bool {
	if ref.LogSet == nil {
		return false
	}
	namespace := ref.LogSet.Namespace
	if namespace == "" {
		namespace = dependentNamespace
	}
	return namespace == key.Namespace && ref.LogSet.Name == key.Name
}

// LogSetReserveOrdinalsChangedPredicate selects updates that change the LogSet
// service-addresses consumed by CNSet and DNSet configuration. Create and delete
// events are intentionally ignored: the owning LogSet already drives dependency
// creation and teardown, and enqueueing dependants from those events would alter
// their existing lifecycle ordering.
func LogSetReserveOrdinalsChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool { return false },
		DeleteFunc: func(event.DeleteEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldSts, oldOK := e.ObjectOld.(*v1beta1.StatefulSet)
			newSts, newOK := e.ObjectNew.(*v1beta1.StatefulSet)
			if !oldOK || !newOK {
				return false
			}
			if _, ok := LogSetStatefulSetOwner(newSts); !ok {
				return false
			}
			return !equality.Semantic.DeepEqual(oldSts.Spec.ReserveOrdinals, newSts.Spec.ReserveOrdinals)
		},
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}
