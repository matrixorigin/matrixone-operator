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
package fake

import (
	"context"
	"github.com/go-logr/logr"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kubefake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func NewContext[T client.Object](obj T, c client.Client, emitter recon.EventEmitter) *recon.Context[T] {
	return &recon.Context[T]{
		Context: context.Background(),
		Obj:     obj,
		Client:  c,
		Log:     logr.Discard(),
		Event:   emitter,
	}
}

var KubeClientBuilder = kubefake.NewClientBuilder
