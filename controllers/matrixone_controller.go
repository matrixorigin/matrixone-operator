package controllers

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/controllers/k8sutils"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// MatrixoneReconciler reconciles a Matrixone object
type MatrixoneReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=matrixone.matrixorigin.cn,resources=matrixones,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=matrixone.matrixorigin.cn,resources=matrixones/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=matrixone.matrixorigin.cn,resources=matrixones/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Matrixone object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *MatrixoneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling Matrixone controller")
	instance := &matrixonev1alpha1.Matrixone{}

	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if err := controllerutil.SetControllerReference(instance, instance, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	err = k8sutils.CreateStandAloneMatrixone(instance)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = k8sutils.CreateStandAloneService(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	reqLogger.Info("Will reconcile matrixone operator in again 10 seconds")

	return ctrl.Result{RequeueAfter: time.Second * 10}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *MatrixoneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&matrixonev1alpha1.Matrixone{}).
		Complete(r)
}
