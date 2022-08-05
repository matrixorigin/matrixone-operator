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
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DNSetActor struct {
	targetNamespacedName types.NamespacedName
	cloneSet             *kruise.CloneSet
}

var _ recon.Actor[*v1alpha1.DNSet] = &DNSetActor{}

func (d *DNSetActor) Observe(ctx *recon.Context[*v1alpha1.DNSet]) (recon.Action[*v1alpha1.DNSet], error) {
	dn := ctx.Obj

	cs := &kruise.CloneSet{}
	err, foundCs := util.IsFound(ctx.Get(client.ObjectKey{Namespace: dn.Namespace, Name: getDNSetName(dn)}, cs))
	if err != nil {
		return nil, errors.Wrap(err, "get dn service cloneset")
	}

	if !foundCs {
		return d.Create, nil
	}
	return nil, nil
}

func (d *DNSetActor) Finalize(ctx *recon.Context[*v1alpha1.DNSet]) (bool, error) {
	dn := ctx.Obj
	var errs error

	svcExit, err := ctx.Exist(client.ObjectKey{Namespace: dn.Namespace, Name: getDNSetHeadlessSvcName(dn)}, &corev1.Service{})
	errs = multierr.Append(errs, err)
	dnSetExit, err := ctx.Exist(client.ObjectKey{Namespace: dn.Namespace, Name: getDNSetName(dn)}, &kruise.CloneSet{})
	errs = multierr.Append(errs, err)

	res := !(svcExit) && (!dnSetExit)

	return res, nil
}

func (d *DNSetActor) Create(ctx *recon.Context[*v1alpha1.DNSet]) error {

	return nil
}
