// Copyright 2022 Matrix Origin
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

package dnset

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	kruise "github.com/openkruise/kruise-api/apps/v1beta1"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Actor struct{}

var _ recon.Actor[*v1alpha1.DNSet] = &Actor{}

type WithResources struct {
	*Actor
	sts *kruise.StatefulSet
}

func (d *Actor) with(sts *kruise.StatefulSet) *WithResources {
	return &WithResources{Actor: d, sts: sts}
}

func (d *Actor) Observe(ctx *recon.Context[*v1alpha1.DNSet]) (recon.Action[*v1alpha1.DNSet], error) {
	dn := ctx.Obj

	sts := &kruise.StatefulSet{}
	err, foundSts := util.IsFound(ctx.Get(client.ObjectKey{
		Namespace: dn.Namespace,
		Name:      stsName(dn),
	}, sts))
	if err != nil {
		return nil, errors.Wrap(err, "get dn service statefulset")
	}

	if !foundSts {
		return d.Create, nil
	}

	origin := sts.DeepCopy()
	if err := syncPods(ctx, sts); err != nil {
		return nil, err
	}
	if !equality.Semantic.DeepEqual(origin, sts) {
		return d.with(sts).Update, nil
	}
	return nil, nil
}

func (d *Actor) Finalize(ctx *recon.Context[*v1alpha1.DNSet]) (bool, error) {
	dn := ctx.Obj

	objs := []client.Object{&corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: headlessSvcName(dn),
	}}, &kruise.StatefulSet{ObjectMeta: metav1.ObjectMeta{
		Name: stsName(dn),
	}}}
	for _, obj := range objs {
		obj.SetNamespace(dn.Namespace)
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

func (d *Actor) Create(ctx *recon.Context[*v1alpha1.DNSet]) error {
	klog.V(recon.Info).Info("create dn set...")
	dn := ctx.Obj

	hSvc := buildHeadlessSvc(dn)
	dnSet := buildDNSet(dn)
	syncReplicas(dn, dnSet)
	syncPodMeta(dn, dnSet)
	syncPodSpec(dn, dnSet, ctx.Dep.Deps.LogSet.Spec.SharedStorage)
	syncPersistentVolumeClaim(dn, dnSet)

	configMap, err := buildDNSetConfigMap(dn, ctx.Dep.Deps.LogSet)
	if err != nil {
		return err
	}

	if err := common.SyncConfigMap(ctx, &dnSet.Spec.Template.Spec, configMap); err != nil {
		return err
	}

	// create all resources
	err = lo.Reduce[client.Object, error]([]client.Object{
		hSvc,
		dnSet,
	}, func(errs error, o client.Object, _ int) error {
		err := ctx.CreateOwned(o)
		return multierr.Append(errs, util.Ignore(apierrors.IsAlreadyExists, err))
	}, nil)
	if err != nil {
		return errors.Wrap(err, "create dn service")
	}

	return nil
}

func (r *WithResources) Scale(ctx *recon.Context[*v1alpha1.DNSet]) error {
	return ctx.Patch(r.sts, func() error {
		syncReplicas(ctx.Obj, r.sts)
		return nil
	})
}

func (r *WithResources) Update(ctx *recon.Context[*v1alpha1.DNSet]) error {
	return ctx.Update(r.sts)
}

func (d *Actor) Reconcile(mgr manager.Manager) error {
	err := recon.Setup[*v1alpha1.DNSet](&v1alpha1.DNSet{}, "dnset", mgr, d,
		recon.WithBuildFn(func(b *builder.Builder) {
			b.Owns(&kruise.StatefulSet{}).
				Owns(&corev1.Service{})
		}))
	if err != nil {
		return err
	}

	return nil
}
