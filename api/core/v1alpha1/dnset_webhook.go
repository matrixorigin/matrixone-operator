package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *DNSet) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-dnset,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=dnsets,verbs=create;update,versions=v1alpha1,name=mdnset.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &DNSet{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *DNSet) Default() {
	r.Spec.DNSetBasic.Default()
}

func (r *DNSetBasic) Default() {}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-dnset,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=dnsets,verbs=create;update,versions=v1alpha1,name=vdnset.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &DNSet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *DNSet) ValidateCreate() error {
	var errs field.ErrorList
	errs = append(errs, validateLogSetRef(&r.Deps.LogSetRef, field.NewPath("deps"))...)
	errs = append(errs, r.Spec.DNSetBasic.ValidateCreate()...)
	errs = append(errs, validateMainContainer(&r.Spec.MainContainer, field.NewPath("spec"))...)
	return invalidOrNil(errs, r)
}

func (r *DNSet) ValidateUpdate(old runtime.Object) error {
	if err := r.ValidateCreate(); err != nil {
		return err
	}
	return nil
}

func (r *DNSet) ValidateDelete() error {
	return nil
}

func (r *DNSetBasic) ValidateCreate() field.ErrorList {
	var errs field.ErrorList
	if r.CacheVolume != nil {
		errs = append(errs, validateVolume(r.CacheVolume, field.NewPath("spec").Child("cacheVolume"))...)
	}
	return errs
}
