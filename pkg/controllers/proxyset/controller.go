// Copyright 2023 Matrix Origin
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

package proxyset

import (
	"fmt"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Actor struct{}

var _ recon.Actor[*v1alpha1.ProxySet] = &Actor{}

func (r *Actor) Observe(ctx *recon.Context[*v1alpha1.ProxySet]) (recon.Action[*v1alpha1.ProxySet], error) {
	p := ctx.Obj
	cloneset := buildCloneSet(p)
	err := recon.CreateOwnedOrUpdate(ctx, cloneset, func() error {
		return syncCloneSet(ctx, p, cloneset)
	})
	if err != nil {
		return nil, errors.Wrap(err, "sync cloneset")
	}
	svc := buildSvc(p)
	err = recon.CreateOwnedOrUpdate(ctx, svc, func() error {
		syncSvc(p, svc)
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "sync service")
	}
	if cloneset.Status.ReadyReplicas >= p.Spec.Replicas {
		p.Status.SetCondition(metav1.Condition{
			Type:    recon.ConditionTypeReady,
			Status:  metav1.ConditionTrue,
			Message: "proxy ready",
		})
		return nil, nil
	}
	// proxy not ready
	msg := fmt.Sprintf("proxy not ready, ready replicas: %d, desired replicas: %d", cloneset.Status.ReadyReplicas, p.Spec.Replicas)
	p.Status.SetCondition(metav1.Condition{
		Type:    recon.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Message: msg,
	})
	return nil, recon.ErrReSync(msg)
}

func (r *Actor) Finalize(ctx *recon.Context[*v1alpha1.ProxySet]) (bool, error) {
	p := ctx.Obj
	objs := []client.Object{
		&corev1.Service{ObjectMeta: serviceKey(p)},
		&kruisev1alpha1.CloneSet{ObjectMeta: cloneSetKey(p)},
	}
	for _, obj := range objs {
		if err := util.Ignore(apierrors.IsNotFound, ctx.Delete(obj)); err != nil {
			return false, err
		}
	}
	for _, obj := range objs {
		exist, err := ctx.Exist(client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return false, err
		}
		if exist {
			return false, nil
		}
	}
	return true, nil
}

func (r *Actor) Reconcile(mgr manager.Manager) error {
	return recon.Setup[*v1alpha1.ProxySet](&v1alpha1.ProxySet{}, "proxyset", mgr, r,
		recon.WithBuildFn(func(b *builder.Builder) {
			b.Owns(&kruisev1alpha1.CloneSet{})
		}))
}
