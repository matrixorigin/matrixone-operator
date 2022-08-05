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

package cnset

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

type CNSetActor struct {
	targetNamespacedName types.NamespacedName
	cloneSet             *kruise.CloneSet
}

var _ recon.Actor[*v1alpha1.CNSet] = &CNSetActor{}

func (c *CNSetActor) Observe(ctx *recon.Context[*v1alpha1.CNSet]) (recon.Action[*v1alpha1.CNSet], error) {
	return nil, nil
}

func (c *CNSetActor) Finalize(ctx *recon.Context[*v1alpha1.CNSet]) (bool, error) {

	return true, nil
}

func (c *CNSetActor) Create(ctx *recon.Context[*v1alpha1.CNSet]) error {

	return nil
}
