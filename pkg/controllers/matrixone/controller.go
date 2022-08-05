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

package matrixone

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/cnset"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/dnset"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/logset"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
)

type MatrixOneActor struct {
	dnset.DNSetActor
	logset.LogSetActor
	cnset.CNSetActor
}

var _ recon.Actor[*v1alpha1.MatrixoneCluster] = &MatrixOneActor{}

func (m *MatrixOneActor) Observe(ctx *recon.Context[*v1alpha1.MatrixoneCluster]) (recon.Action[*v1alpha1.MatrixoneCluster], error) {
	return nil, nil
}

func (m *MatrixOneActor) Finalize(ctx *recon.Context[*v1alpha1.MatrixoneCluster]) (bool, error) {
	return true, nil
}
