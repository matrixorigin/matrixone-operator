// Copyright 2021 Matrix Origin
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

package matrixone

import (
	"fmt"

	"github.com/matrixorigin/matrixone-operator/pkg/controllers/utils"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// All methods to implement GenericPredicates type
// GenericPredicates to be passed to manager
type GenericPredicates struct {
	predicate.Funcs
}

// create() to filter create events
func (GenericPredicates) Create(e event.CreateEvent) bool {
	return IgnoreNamespacePredicate(e.Object)
}

// update() to filter update events
func (GenericPredicates) Update(e event.UpdateEvent) bool {
	return IgnoreNamespacePredicate(e.ObjectNew)

}

func IgnoreNamespacePredicate(obj object) bool {
	namespaces := utils.GetEnvAsSlice("DENY_LIST", nil, ",")

	for _, namespace := range namespaces {
		if obj.GetNamespace() == namespace {
			msg := fmt.Sprintf("matrixone operator will not re-concile namespace [%s], alter DENY_LIST to re-concile", obj.GetNamespace())
			logger.Info(msg)
			return false
		}
	}
	return true
}
