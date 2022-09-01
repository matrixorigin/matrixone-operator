package v1alpha1

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var webhookLog = logf.Log.WithName("mo-webhook")

func RegisterWebhooks(mgr ctrl.Manager) error {
	if err := (&MatrixOneCluster{}).setupWebhookWithManager(mgr); err != nil {
		return err
	}
	if err := (&LogSet{}).setupWebhookWithManager(mgr); err != nil {
		return err
	}
	if err := (&DNSet{}).setupWebhookWithManager(mgr); err != nil {
		return err
	}
	if err := (&CNSet{}).setupWebhookWithManager(mgr); err != nil {
		return err
	}
	return nil
}

func invalidOrNil(allErrs field.ErrorList, r client.Object) error {
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(r.GetObjectKind().GroupVersionKind().GroupKind(), r.GetName(), allErrs)
}

func validateLogSetRef(ref *LogSetRef, parent *field.Path) field.ErrorList {
	var errs field.ErrorList
	if ref.LogSet == nil && ref.ExternalLogSet == nil {
		errs = append(errs, field.Invalid(parent, nil, "one of deps.logSet or deps.externalLogSet must be set"))
	}
	return errs
}

func validateMainContainer(c *MainContainer, parent *field.Path) field.ErrorList {
	var errs field.ErrorList
	if c.Image == "" {
		errs = append(errs, field.Invalid(parent.Child("image"), c.Image, "image must be set"))
	}
	return errs
}
