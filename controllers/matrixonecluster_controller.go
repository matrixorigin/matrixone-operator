/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"os"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1"
)

// MatrixoneClusterReconciler reconciles a MatrixoneCluster object
type MatrixoneClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	ReconcileWait time.Duration
	Recorder      record.EventRecorder
}

func NewMatrixoneReconciler(mgr ctrl.Manager) *MatrixoneClusterReconciler {
	return &MatrixoneClusterReconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("Matrixone"),
		Scheme:        mgr.GetScheme(),
		ReconcileWait: LookupReconcileTime(),
		Recorder:      mgr.GetEventRecorderFor("matrixone-operator"),
	}
}

//+kubebuilder:rbac:groups=matrixone.matrixorigin.cn,resources=matrixoneclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=matrixone.matrixorigin.cn,resources=matrixoneclusters/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MatrixoneCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *MatrixoneClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	// _ = r.Log.WithValues("matrixone", req.NamespacedName)
	matrixone := &matrixonev1alpha1.MatrixoneCluster{}
	ls := map[string]string{"matrixone": matrixone.Name}
	klog.Info("matrixone...", matrixone.Name)

	logger.Info("Starting recon")

	// Get the matrixone resource
	if err := r.Get(ctx, req.NamespacedName, matrixone); err != nil {
		logger.Error(err, "unable to fetch matrixone")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("Create Service resource")
	desiredSer, err := r.makeService(&matrixone.Spec.Services, matrixone, ls)
	if err != nil {
		logger.Error(err, "unable to make Service")
	}
	SerapplyOpts := []client.PatchOption{client.ForceOwnership, client.FieldOwner("matrixone-controller")}
	err = r.Patch(ctx, desiredSer, client.Apply, SerapplyOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}
	matrixone.Status.ServiceStatus = desiredSer.Status

	logger.Info("Create ConfigMap resource")
	_, err = r.makeConfigMap(&matrixone.Spec.ConfigMap, matrixone, ls)
	if err != nil {
		logger.Error(err, "unable to make ConfigMap")
	}
	CMapplyOpts := []client.PatchOption{client.ForceOwnership, client.FieldOwner("matrixone-controller")}
	err = r.Patch(ctx, desiredSer, client.Apply, CMapplyOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Create StatefulSet resource")
	// Create StatefulSet
	desiredSS, err := r.makeStatefulset(matrixone, ls)
	if err != nil {
		logger.Error(err, "unable to make Statefulset")
		return ctrl.Result{}, err
	}

	logger.Info("Server side apply of this spec")
	// Patch the statefulSet
	applyOpts := []client.PatchOption{client.ForceOwnership, client.FieldOwner("matrixone-controller")}
	err = r.Patch(ctx, &desiredSS, client.Apply, applyOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Update the matrixone status with the StatefulSet status")
	// Update Status
	matrixone.Status.SSStatus = desiredSS.Status

	err = r.Status().Update(ctx, matrixone)
	if err != nil {
		logger.Info("Error while updating status", "Error", err)
		return ctrl.Result{}, err
	}

	logger.Info("Recon successful!")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MatrixoneClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&matrixonev1alpha1.MatrixoneCluster{}).
		Owns(&appsv1.StatefulSet{}).
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
