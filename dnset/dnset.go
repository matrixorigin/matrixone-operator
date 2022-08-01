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
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

type DNSetController struct {
	targetNamespacedName types.NamespacedName
	cloneSet             *kruise.CloneSet
	recon                recon.Actor[*v1alpha1.DNSet]
}

var _ CommonController = &DNSetController{}

func (d *DNSetController) Create() error {
	return nil
}

func (d *DNSetController) Finialize() error {
	return nil
}

func (d *DNSetController) size(ctx recon.Context[*v1alpha1.DNSet]) (int32, error) {
	if d.cloneSet == nil {
		err := d.fetchCloneSet(ctx)
		if err != nil {
			return 0, err
		}
	}
	if d.cloneSet.Spec.Replicas == nil {
		return 1, nil
	}

	return *d.cloneSet.Spec.Replicas, nil
}

func (d *DNSetController) fetchCloneSet(ctx recon.Context[*v1alpha1.DNSet]) error {
	workload := kruise.CloneSet{}
	err := ctx.Client.Get(ctx, d.targetNamespacedName, &workload)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			ctx.Event.EmitEventGeneric(string(CloneSetGetError), "fetch clone set error", err)
		}
	}

	d.cloneSet = &workload

	return nil
}
