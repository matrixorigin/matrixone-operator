package controllers

import (
	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	port int32 = 6000
)

func (r *MatrixoneClusterReconciler) makeService(svc *v1.Service, moc *matrixonev1alpha1.MatrixoneCluster, ls map[string]string) (*v1.Service, error) {
	svc.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Service",
	}

	svc.ObjectMeta = metav1.ObjectMeta{
		Name:      moc.Name,
		Namespace: moc.Namespace,
		Labels:    ls,
	}

	svc.Spec.Selector = ls
	svc.Spec.Ports = []v1.ServicePort{
		{
			Name:       "mo-port",
			Port:       port,
			TargetPort: intstr.FromInt(int(port)),
		},
	}

	if err := ctrl.SetControllerReference(moc, svc, r.Scheme); err != nil {
		return svc, err
	}

	return svc, nil
}
