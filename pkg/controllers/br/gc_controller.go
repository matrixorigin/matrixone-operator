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

package br

import (
	"github.com/matrixorigin/controller-runtime/pkg/observer"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"
)

type GCActor[T JobObject] struct {
	ConditionType recon.ConditionType
}

func (r *GCActor[T]) Observe(c *recon.Context[T]) error {
	cond, ok := recon.GetCondition(c.Obj, r.ConditionType)
	if !ok || cond.Status == metav1.ConditionFalse {
		// not completed, nothing to do
		return nil
	}
	sinceComplete := time.Now().Sub(cond.LastTransitionTime.Time)
	if sinceComplete > c.Obj.GetTTL() {
		return c.Delete(c.Obj)
	}
	return recon.ErrReSync("wait for ttl", c.Obj.GetTTL()-sinceComplete)
}

func StartJobGCer[T JobObject](mgr manager.Manager, actor *GCActor[T], obj T) error {
	return observer.Setup[T](
		obj,
		obj.GetObjectKind().GroupVersionKind().Kind+"-gc",
		mgr,
		actor,
		recon.WithPredicate(predicate.ResourceVersionChangedPredicate{}))
}
