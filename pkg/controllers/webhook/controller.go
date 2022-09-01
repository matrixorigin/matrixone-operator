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

type WebhookType string

const (
	WebhookTypeValidating = "Validating"
	WebhookTypeMutating   = "Mutating"
)

const (
	CaInjectionAnnoKey = "matrixorigin.io/ca-injection"
)

type WebhookController struct {
	Client   client.Client
	Type     WebhookType
	CaBundle []byte
	Logger   logr.Logger
}

func (c *WebhookController) Reconcile(ctx context.Context, req recon.Request) (recon.Result, error) {
	switch c.Type {
	case WebhookTypeValidating:
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
	case WebhookTypeMutating:
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

func Setup(typ WebhookType, mgr ctrl.Manager, caBundle []byte) error {
	c := &WebhookController{
		Client:   mgr.GetClient(),
		Type:     typ,
		CaBundle: caBundle,
		Logger:   mgr.GetLogger().WithName("webhook-controller"),
	}
	builder := ctrl.NewControllerManagedBy(mgr)
	switch c.Type {
	case WebhookTypeValidating:
		builder = builder.For(&v1.ValidatingWebhookConfiguration{})
	case WebhookTypeMutating:
		builder = builder.For(&v1.MutatingWebhookConfiguration{})
	default:
		return errors.Errorf("unkown webhook type %s", c.Type)
	}
	return builder.Complete(c)
}
