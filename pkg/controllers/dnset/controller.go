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
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/utils"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type DNSetActor struct{}

var _ recon.Actor[*v1alpha1.DNSet] = &DNSetActor{}

type WithResources struct {
	*DNSetActor
	cloneSet *kruise.CloneSet
}

func (d *DNSetActor) Observe(ctx *recon.Context[*v1alpha1.DNSet]) (recon.Action[*v1alpha1.DNSet], error) {
	ds := ctx.Obj

	svc := &corev1.Service{}
	err, foundSvc := util.IsFound(ctx.Get(client.ObjectKey{Namespace: ds.Namespace, Name: ds.Name}, svc))
	if err != nil {
		return nil, errors.Wrap(err, "get dn service discovery service")
	}

	cloneSet := &kruise.CloneSet{}
	err, foundCs := util.IsFound(ctx.Get(client.ObjectKey{Namespace: ds.Namespace, Name: ds.Name}, cloneSet))
	if err != nil {
		return nil, errors.Wrap(err, "get dn service cloneset")
	}

	if !foundCs || !foundSvc {
		return d.Create, nil
	}
	return nil, nil
}

func (d *DNSetActor) Finalize(ctx *recon.Context[*v1alpha1.DNSet]) (bool, error) {
	dn := ctx.Obj
	var errs error

	svcExit, err := ctx.Exist(client.ObjectKey{Namespace: dn.Namespace, Name: dn.Name}, &corev1.Service{})
	err = multierr.Append(errs, err)

	hSvcExit, err := ctx.Exist(client.ObjectKey{
		Namespace: dn.Namespace, Name: utils.GetHeadlessSvcName(dn)},
		&corev1.Service{})
	errs = multierr.Append(errs, err)

	dnSetExit, err := ctx.Exist(client.ObjectKey{
		Namespace: dn.Namespace, Name: utils.GetHeadlessSvcName(dn)},
		&kruise.CloneSet{})
	errs = multierr.Append(errs, err)

	res := !hSvcExit && !dnSetExit && !svcExit

	return res, nil
}

func (d *DNSetActor) Create(ctx *recon.Context[*v1alpha1.DNSet]) error {
	klog.V(recon.Info).Info("create dn set...")
	ds := ctx.Obj

	hSvc := buildHeadlessSvc(ds)
	dsCloneSet := buildDNSet(ds)
	svc := buildSvc(ds)
	syncReplicas(ds, dsCloneSet)
	syncPodMeta(ds, dsCloneSet)
	syncPodSpec(ds, dsCloneSet)
	syncPersistentVolumeClaim(ds, dsCloneSet)
	configMap, err := buildDNSetConfigMap(ds)
	if err != nil {
		return err
	}

	if err := common.SyncConfigMap(ctx, &dsCloneSet.Spec.Template.Spec, configMap); err != nil {
		return err
	}

	// create all resources
	err = lo.Reduce[client.Object, error]([]client.Object{
		hSvc,
		svc,
		dsCloneSet,
	}, func(errs error, o client.Object, _ int) error {
		err := ctx.CreateOwned(o)
		return multierr.Append(errs, util.Ignore(apierrors.IsAlreadyExists, err))
	}, nil)
	if err != nil {
		return errors.Wrap(err, "create dn service")
	}

	return nil
}

func (d *DNSetActor) Reconcile(mgr manager.Manager, dn *v1alpha1.DNSet) error {
	err := recon.Setup[*v1alpha1.DNSet](dn, "dn set", mgr, d)
	if err != nil {
		return err
	}

	return nil
}
