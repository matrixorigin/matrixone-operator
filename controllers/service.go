package controllers

import (
	"fmt"

	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func makeService(svc *v1.Service, moc *matrixonev1alpha1.MatrixoneCluster, ls map[string]string, uniqueStr string) (*v1.Service, error) {
	svc.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Service",
	}

	svc.ObjectMeta.Name = getServiceName(svc.ObjectMeta.Name, uniqueStr)

	svc.ObjectMeta.Namespace = moc.Namespace

	if svc.ObjectMeta.Labels == nil {
		svc.ObjectMeta.Labels = ls
	} else {
		for k, v := range ls {
			svc.ObjectMeta.Labels[k] = v
		}
	}

	if svc.Spec.Selector == nil {
		svc.Spec.Selector = ls
	} else {
		for k, v := range ls {
			svc.Spec.Selector[k] = v
		}
	}

	if svc.Spec.Ports == nil {
		svc.Spec.Ports = []v1.ServicePort{
			{
				Name:       "service-port",
				Port:       moc.Spec.MatrixonePort,
				TargetPort: intstr.FromInt(int(moc.Spec.MatrixonePort)),
			},
		}
	}

	return svc, nil
}

func getServiceName(nameTemplate, uniqueStr string) string {
	if nameTemplate == "" {
		return uniqueStr
	} else {
		return fmt.Sprintf(nameTemplate, uniqueStr)
	}
}

func makeServiceEmptyObj() *v1.Service {
	return &v1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
	}
}
