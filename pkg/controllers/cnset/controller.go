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

package cnset

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type CNSetActor struct{}

var _ recon.Actor[*v1alpha1.CNSet] = &CNSetActor{}

func (c *CNSetActor) Observe(ctx *recon.Context[*v1alpha1.CNSet]) (recon.Action[*v1alpha1.CNSet], error) {
	cn := ctx.Obj

	svc := &corev1.Service{}
	err, foundSvc := util.IsFound(ctx.Get(client.ObjectKey{
		Namespace: common.GetNamespace(cn),
		Name:      common.GetName(cn)}, svc))
	if err != nil {
		return nil, errors.Wrap(err, "get dn service discovery service")
	}

	cloneSet := &kruise.CloneSet{}
	err, foundCs := util.IsFound(ctx.Get(client.ObjectKey{
		Namespace: common.GetNamespace(cn), Name: common.GetName(cn)}, cloneSet))
	if err != nil {
		return nil, errors.Wrap(err, "get dn service cloneset")
	}

	if !foundCs || !foundSvc {
		return c.Create, nil
	}
	return nil, nil

}

func (c *CNSetActor) Finalize(ctx *recon.Context[*v1alpha1.CNSet]) (bool, error) {
	cn := ctx.Obj

	objs := []client.Object{&corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: common.GetHeadlessSvcName(cn),
	}}, &kruise.CloneSet{ObjectMeta: metav1.ObjectMeta{
		Name: common.GetName(cn),
	}}, &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: common.GetDiscoverySvcName(cn),
	}}}
	for _, obj := range objs {
		obj.SetNamespace(common.GetNamespace(cn))
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

func (c *CNSetActor) Create(ctx *recon.Context[*v1alpha1.CNSet]) error {
	klog.V(recon.Info).Info("dn set create...")
	cn := ctx.Obj

	hSvc := buildHeadlessSvc(cn)
	cnCloneSet := buildCNSet(cn)
	svc := buildSvc(cn)
	syncReplicas(cn, cnCloneSet)
	syncPodMeta(cn, cnCloneSet)
	syncPodSpec(cn, cnCloneSet)
	syncPersistentVolumeClaim(cn, cnCloneSet)
	configMap, err := buildCNSetConfigMap(cn)
	if err != nil {
		return err
	}

	if err := common.SyncConfigMap(ctx, &cnCloneSet.Spec.Template.Spec, configMap); err != nil {
		return err
	}

	// create all resources
	err = lo.Reduce[client.Object, error]([]client.Object{
		hSvc,
		svc,
		cnCloneSet,
	}, func(errs error, o client.Object, _ int) error {
		err := ctx.CreateOwned(o)
		return multierr.Append(errs, util.Ignore(apierrors.IsAlreadyExists, err))
	}, nil)
	if err != nil {
		return errors.Wrap(err, "create dn service")
	}
	return nil
}

func (c *CNSetActor) Reconcile(mgr manager.Manager, cn *v1alpha1.CNSet) error {
	err := recon.Setup[*v1alpha1.CNSet](cn, "cnset", mgr, c,
		recon.WithBuildFn(func(b *builder.Builder) {
			b.Owns(&kruise.CloneSet{}).
				Owns(&corev1.Service{})
		}))
	if err != nil {
		return err
	}

	return nil
}
