package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *CNSet) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-cnset,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=cnsets,verbs=create;update,versions=v1,name=mcnset.kb.io,admissionReviewVersions={v1,v1beta1}
var _ webhook.Defaulter = &CNSet{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *CNSet) Default() {
	if r.Spec.ServiceType == "" {
		r.Spec.ServiceType = corev1.ServiceTypeClusterIP
	}
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-cnset,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=cnsets,verbs=create;update,versions=v1,name=vcnset.kb.io,admissionReviewVersions={v1,v1beta1}
var _ webhook.Validator = &CNSet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *CNSet) ValidateCreate() error {
	var errs field.ErrorList
	errs = append(errs, validateLogSetRef(&r.Deps.LogSetRef, field.NewPath("deps"))...)
	errs = append(errs, validateMainContainer(&r.Spec.MainContainer, field.NewPath("spec"))...)
	return invalidOrNil(errs, r)
}

func (r *CNSet) ValidateUpdate(old runtime.Object) error {
	if err := r.ValidateCreate(); err != nil {
		return err
	}
	return nil
}

func (r *CNSet) ValidateDelete() error {
	return nil
}
