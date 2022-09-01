package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var moLog = logf.Log.WithName("mo-cluster")

func (r *MatrixOneCluster) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-matrixonecluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=matrixoneclusters,verbs=create;update,versions=v1alpha1,name=mmatrixonecluster.kb.io,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &MatrixOneCluster{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *MatrixOneCluster) Default() {
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-matrixonecluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=matrixoneclusters,verbs=create;update,versions=v1alpha1,name=vmatrixonecluster.kb.io,admissionReviewVersions=v1;v1beta1

var _ webhook.Validator = &MatrixOneCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MatrixOneCluster) ValidateCreate() error {
	return nil
}

func (r *MatrixOneCluster) ValidateUpdate(old runtime.Object) error {
	return nil
}

func (r *MatrixOneCluster) ValidateDelete() error {
	return nil
}
