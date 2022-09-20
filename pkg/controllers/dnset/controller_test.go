// Copyright 2022 Matrix Origin
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
