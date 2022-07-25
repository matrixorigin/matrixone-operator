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
