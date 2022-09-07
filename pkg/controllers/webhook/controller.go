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
package webhook

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	v1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	recon "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Type string

const (
	TypeValidating = "Validating"
	TypeMutating   = "Mutating"
)

const (
	CaInjectionAnnoKey = "matrixorigin.io/ca-injection"
)

type Controller struct {
	Client   client.Client
	Type     Type
	CaBundle []byte
	Logger   logr.Logger
}

func (c *Controller) Reconcile(ctx context.Context, req recon.Request) (recon.Result, error) {
	switch c.Type {
	case TypeValidating:
		hook := &v1.ValidatingWebhookConfiguration{}
		err := c.Client.Get(ctx, req.NamespacedName, hook)
		if err != nil {
			return recon.Result{}, err
		}
		if _, ok := hook.Annotations[CaInjectionAnnoKey]; !ok {
			return recon.Result{}, nil
		}
		old := hook.DeepCopy()
		for i := range hook.Webhooks {
			hook.Webhooks[i].ClientConfig.CABundle = c.CaBundle
		}
		if !equality.Semantic.DeepEqual(old, hook) {
			c.Logger.Info("update webhook", "webhook", req.NamespacedName)
			return recon.Result{}, c.Client.Update(ctx, hook)
		}
		return recon.Result{}, nil
	case TypeMutating:
		// sadly MutatingWebhook and ValidatingWebhook cannot be unified via generic
		hook := &v1.MutatingWebhookConfiguration{}
		err := c.Client.Get(ctx, req.NamespacedName, hook)
		if err != nil {
			return recon.Result{}, err
		}
		if _, ok := hook.Annotations[CaInjectionAnnoKey]; !ok {
			return recon.Result{}, nil
		}
		old := hook.DeepCopy()
		for i := range hook.Webhooks {
			hook.Webhooks[i].ClientConfig.CABundle = c.CaBundle
		}
		if !equality.Semantic.DeepEqual(old, hook) {
			c.Logger.Info("update webhook", "webhook", req.NamespacedName)
			return recon.Result{}, c.Client.Update(ctx, hook)
		}
		return recon.Result{}, nil
	default:
		return recon.Result{}, nil
	}
}

func Setup(typ Type, mgr ctrl.Manager, caBundle []byte) error {
	c := &Controller{
		Client:   mgr.GetClient(),
		Type:     typ,
		CaBundle: caBundle,
		Logger:   mgr.GetLogger().WithName("webhook-controller"),
	}
	builder := ctrl.NewControllerManagedBy(mgr)
	switch c.Type {
	case TypeValidating:
		builder = builder.For(&v1.ValidatingWebhookConfiguration{})
	case TypeMutating:
		builder = builder.For(&v1.MutatingWebhookConfiguration{})
	default:
		return errors.Errorf("unkown webhook type %s", c.Type)
	}
	return builder.Complete(c)
}
