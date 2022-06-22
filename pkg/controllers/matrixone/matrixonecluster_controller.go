// Copyright 2021 Matrix Origin
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

package matrixone

import (
	"context"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/matrixorigin/matrixone-operator/pkg/actor"
	"github.com/matrixorigin/matrixone-operator/pkg/apis/matrixone/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/state"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterReconciler reconciles a MatrixoneCluster object
type ClusterReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
	ReconcileWait time.Duration
	Recorder      record.EventRecorder
	Actor         actor.EventActor
	StateHandler  state.ObjStateTransFunc
}

func NewMatrixoneReconciler(mgr ctrl.Manager) *ClusterReconciler {
	return &ClusterReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Log:           ctrl.Log.WithName("controllers").WithName("matrixone"),
		ReconcileWait: LookupReconcileTime(),
		Recorder:      mgr.GetEventRecorderFor("matrixone-operator"),
	}
}

//+kubebuilder:rbac:groups=matrixone.matrixorigin.cn,resources=matrixoneclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=matrixone.matrixorigin.cn,resources=matrixoneclusters/status,verbs=get;update;patch

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
	var emitEvent EventEmitter = EmitEventFuncs{r.Recorder, r.Actor, r.StateHandler}

	if err := deployMatrixoneCluster(r.Client, instance, emitEvent); err != nil {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: r.ReconcileWait}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.MatrixoneCluster{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.StatefulSet{}).
		WithEventFilter(GenericPredicates{}).
		Complete(r)
}

func LookupReconcileTime() time.Duration {
	val, exists := os.LookupEnv("RECONCILE_WAIT")

	if !exists {
		return time.Second * 10
	}
	v, err := time.ParseDuration(val)
	if err != nil {
		logger.Error(err, err.Error())

		os.Exit(1)
	}
	return v
}
