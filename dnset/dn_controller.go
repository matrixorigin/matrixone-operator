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
	kruisev1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
)

type DNSetActor struct{}
type WithResources struct{
	*DNSetActor
	cloneSet *kruisev1alpha1.CloneSet
}

var _ recon.Actor[*v1alpha1.DNSet] = &DNSetActor{}

func (r *DNSetActor) with(cs *kruisev1alpha1.CloneSet) *WithResources {
	return &WithResources{DNSetActor: r, cloneSet: cs}
}

// Observe: observe dnset bootstrap
func (d *DNSetActor) Observe(ctx *recon.Context[*v1alpha1.DNSet]) (recon.Action[*v1alpha1.DNSet], error) {

	return nil, nil
}

// Create: create dn pod
func (d *DNSetActor) Create(ctx *recon.Context[*v1alpha1.DNSet]) error {
	return nil
}

// Finalize: finalize dnset
func (d *DNSetActor) Finalize(ctx *recon.Context[*v1alpha1.DNSet]) (bool, error) {
	return true, nil
}

// Bootstrap: bootstrap dnset
func (d *DNSetActor) Bootstrap(ctx *recon.Context[*v1alpha1.DNSet]) error {
	return nil
}

// Scale: scale in/scale out dnset
// type: Horizontal scaling and Vertical scaling
func (w *WithResources) Scale(ctx *recon.Context[*v1alpha1.DNSet]) error {
	return nil
}

// Repair: repair dnset
func (w *WithResources) Repair(ctx *recon.Context[*v1alpha1.DNSet]) error {
	return nil
}

// Update: update dnset
func (w *WithResources) Update(ctx *recon.Context[*v1alpha1.DNSet]) error {
	return nil
}
