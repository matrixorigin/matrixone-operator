// Copyright 2024 Matrix Origin
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

package hook

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type CNClaimSetHook struct {
	cli    client.Client
	logger logr.Logger
}

func NewCNClaimSetHook(cli client.Client, logger logr.Logger) *CNClaimSetHook {
	return &CNClaimSetHook{
		cli:    cli,
		logger: logger,
	}
}

func (h *CNClaimSetHook) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

func (h *CNClaimSetHook) ValidateCreate(_ context.Context, _ runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (h *CNClaimSetHook) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	oldCS, ok := oldObj.(*v1alpha1.CNClaimSet)
	if !ok {
		return nil, nil
	}
	newCS, ok := newObj.(*v1alpha1.CNClaimSet)
	if !ok {
		return nil, nil
	}
	// scale from zero case
	// TODO(aylei): optimize
	if oldCS.Spec.Replicas == 0 && newCS.Spec.Replicas > 0 {
		return nil, nil
	}
	return nil, nil
}

func (h *CNClaimSetHook) ValidateDelete(_ context.Context, _ runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (h *CNClaimSetHook) Setup(mgr manager.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.CNClaimSet{}).
		WithValidator(h).
		Complete()
}
