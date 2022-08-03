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
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	kruise "github.com/openekruise/kruise-api/apps/v1alpha1"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DNSetActor struct {
	targetNamespacedName types.NamespacedName
	cloneSet             *kruise.ClonetSet
}

var _ recon.Actor[*v1alpha1.DNSet] = &DNSetActor{}

func (d *DNSetActor) Observe(ctx *recon.Context[*v1alpha1.DNSet]) (recon.Action[*v1alpha1.DNSet], error) {
	ds := ctx.Obj

	cs := &kruise.CloneSet{}
	err, foundCs := util.IsFound(ctx.Get(client.ObjectKey{Namespace: ds.Namespace, Name: getName(ds)}, cs))
	if err != nil {
		return nil, errors.Wrap(err, "get dn service cloneset")
	}

	if !foundCs {
		return d.Create, nil
	}
	return nil, nil
}

func (d *DNSetActor) Finalize(ctx *recon.Context[*v1alpha1.DNSet]) (bool, error) {
	ds := ctx.Obj
	var errs error

	svcExit, err := ctx.Exist(client.ObjectKey{Namespace: ds.Namespace, Name: headlessSvcName(ds)}, &corev1.Service{})
	errs = multierr.Append(errs, err)
	dnSetExit, err := ctx.Exist(client.ObjectKey{Namespace: ds.Namespace, Name: getName(ds)}, &kruise.CloneSet{})
	errs = multierr.Append(errs, err)

	res := !(svcExit) && (!dnSetExit)

	return res, nil
}

func (d *DNSetActor) Create(ctx *recon.Context[*v1alpha1.DNSet]) error {

	return nil
}

func (d *DNSetActor) size(ctx *recon.Context[*v1alpha1.DNSet]) (int32, error) {
	if d.cloneSet == nil {
		err := d.fetchCloneSet(ctx)
		if err != nil {
			return 0, err
		}
	}

	// default is 1
	if d.cloneSet.Spec.Replicas == nil {
		return 1, nil
	}

	return *d.cloneSet.Spec.Replicas, nil
}

func (d *DNSetActor) fetchCloneSet(ctx *recon.Context[*v1alpha1.DNSet]) error {
	cs := kruise.CloneSet{}
	err := ctx.Client.Get(ctx.Context, d.targetNamespacedName, &cs)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			ctx.Event.EmitEventGeneric(string(FetchFail), "clonsetfetch error", err)
		}
		return err
	}

	d.cloneSet = &cs

	return nil
}
