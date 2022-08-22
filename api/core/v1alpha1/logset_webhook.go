package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const (
	defaultShardNum = 1

	minHAReplicas = 3
	singleReplica = 1
)

func (r *LogSet) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-logset,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=logsets,verbs=create;update,versions=v1,name=mlogset.kb.io,admissionReviewVersions={v1,v1beta1}
var _ webhook.Defaulter = &LogSet{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *LogSet) Default() {
	if r.Spec.InitialConfig.HAKeeperReplicas == nil {
		if r.Spec.Replicas >= minHAReplicas {
			r.Spec.InitialConfig.HAKeeperReplicas = pointer.Int(minHAReplicas)
		}
		r.Spec.InitialConfig.HAKeeperReplicas = pointer.Int(singleReplica)
	}
	if r.Spec.InitialConfig.LogShardReplicas == nil {
		if r.Spec.Replicas >= minHAReplicas {
			r.Spec.InitialConfig.LogShardReplicas = pointer.Int(minHAReplicas)
		}
		r.Spec.InitialConfig.LogShardReplicas = pointer.Int(singleReplica)
	}
	if r.Spec.InitialConfig.LogShards == nil {
		r.Spec.InitialConfig.LogShards = pointer.Int(defaultShardNum)
	}
	if r.Spec.InitialConfig.DNShards == nil {
		r.Spec.InitialConfig.DNShards = pointer.Int(defaultShardNum)
	}
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-logset,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=logsets,verbs=create;update,versions=v1,name=vlogset.kb.io,admissionReviewVersions={v1,v1beta1}
var _ webhook.Validator = &LogSet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *LogSet) ValidateCreate() error {
	var errs field.ErrorList
	errs = append(errs, validateMainContainer(&r.Spec.MainContainer, field.NewPath("spec"))...)
	return nil
}

func (r *LogSet) ValidateUpdate(old runtime.Object) error {
	return nil
}

func (r *LogSet) ValidateDelete() error {
	return nil
}

func (r *LogSet) validateInitialConfig() error {
	var errs field.ErrorList
	parent := field.NewPath("spec").Child("initialConfig")

	if hrs := r.Spec.InitialConfig.HAKeeperReplicas; hrs == nil {
		errs = append(errs, field.Invalid(parent.Child("haKeeperReplicas"), hrs, "haKeeperReplicas must be set"))
	} else if *hrs > int(r.Spec.Replicas) {
		errs = append(errs, field.Invalid(parent.Child("haKeeperReplicas"), hrs, "haKeeperReplicas must not larger then logservice replicas"))
	}

}
