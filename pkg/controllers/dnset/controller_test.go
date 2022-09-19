package dnset

import (
	"testing"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
)

func Test_syncPods(t *testing.T) {
	type args struct {
		ctx *recon.Context[*v1alpha1.DNSet]
		sts *kruisev1.StatefulSet
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := syncPods(tt.args.ctx, tt.args.sts); (err != nil) != tt.wantErr {
				t.Errorf("syncPods() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

}
