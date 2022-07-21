package dnset

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	controllerName = "DNSetController"
)

var log = logf.Log.WithName("DNSet-controller")

type  DNReconcilerConfig struct {}

type DNReconciler struct {
	client.Client
	scheme *runtime.Scheme
	recoder record.EventRecorder
}

func NewReconciler(mgr ctrl.Manager, config DNReconcilerConfig) {

}
