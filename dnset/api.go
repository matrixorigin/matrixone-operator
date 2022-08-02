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
)

// DNSetController is the interface that all types of DNset controller implements
type DNSetCommonController interface {
	Create(ctx recon.Context[*v1alpha1.DNSet]) error
	Scale(ctx recon.Context[*v1alpha1.DNSet]) error
	Update(ctx recon.Context[*v1alpha1.DNSet]) error
}
