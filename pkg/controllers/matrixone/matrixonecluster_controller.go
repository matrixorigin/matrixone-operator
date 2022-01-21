package matrixone

import (
	"context"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/matrixorigin/matrixone-operator/pkg/apis/matrixone/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MatrixoneClusterReconciler reconciles a MatrixoneCluster object
type MatrixoneClusterReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
	ReconcileWait time.Duration
	Recorder      record.EventRecorder
}

func NewMatrixoneReconciler(mgr ctrl.Manager) *MatrixoneClusterReconciler {
	return &MatrixoneClusterReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Log:           ctrl.Log.WithName("controllers").WithName("Matrixone"),
		ReconcileWait: LookupReconcileTime(),
		Recorder:      mgr.GetEventRecorderFor("matrixone-operator"),
	}
}

//+kubebuilder:rbac:groups=matrixone.matrixorigin.cn,resources=matrixoneclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=matrixone.matrixorigin.cn,resources=matrixoneclusters/status,verbs=get;update;patch

func (r *MatrixoneClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("matrixone", req.NamespacedName)

	instance := &v1alpha1.MatrixoneCluster{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
	}

	// Intialize Emit Events
	var emitEvent EventEmitter = EmitEventFuncs{r.Recorder}

	if err := deployMatrixoneCluster(r.Client, instance, emitEvent); err != nil {
		return ctrl.Result{}, nil
	} else {
		return ctrl.Result{RequeueAfter: r.ReconcileWait}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MatrixoneClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.MatrixoneCluster{}).
		WithEventFilter(GenericPredicates{}).
		Complete(r)
}

func LookupReconcileTime() time.Duration {
	val, exists := os.LookupEnv("RECONCILE_WAIT")

	if !exists {
		return time.Second * 10
	} else {
		v, err := time.ParseDuration(val)
		if err != nil {
			logger.Error(err, err.Error())

			os.Exit(1)
		}
		return v
	}
}
