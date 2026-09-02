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
	"github.com/openkruise/kruise-api/apps/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// LogSetStatefulSetChangedPredicate selects events that can change the LogSet
// service-addresses consumed by CNSet and DNSet configuration.
func LogSetStatefulSetChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool { return true },
		DeleteFunc: func(event.DeleteEvent) bool { return true },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldSts, oldOK := e.ObjectOld.(*v1beta1.StatefulSet)
			newSts, newOK := e.ObjectNew.(*v1beta1.StatefulSet)
			return oldOK && newOK && !equality.Semantic.DeepEqual(oldSts.Spec.ReserveOrdinals, newSts.Spec.ReserveOrdinals)
		},
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}
