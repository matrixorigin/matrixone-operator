package controllers

import (
	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var port = []v1.ServicePort{
	{
		Name:       "server",
		Port:       serverPort,
		TargetPort: intstr.FromInt(int(serverPort)),
	},
	{
		Name:       "addr-raft",
		Port:       addrRaftPort,
		TargetPort: intstr.FromInt(int(addrRaftPort)),
	},
	{
		Name:       "addr-client",
		Port:       addrClientPort,
		TargetPort: intstr.FromInt(int(addrClientPort)),
	},
	{
		Name:       "rpc",
		Port:       rpcAddrPort,
		TargetPort: intstr.FromInt(int(rpcAddrPort)),
	},
	{
		Name:       "client",
		Port:       clientPort,
		TargetPort: intstr.FromInt(int(clientPort)),
	},
	{
		Name:       "peer",
		Port:       peerPort,
		TargetPort: intstr.FromInt(int(peerPort)),
	},
	{
		Name:       "raft",
		Port:       raftPort,
		TargetPort: intstr.FromInt(int(raftPort)),
	},
}

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
	svc.Spec.Type = moc.Spec.ServiceType
	svc.Spec.Ports = port

	if err := ctrl.SetControllerReference(moc, svc, r.Scheme); err != nil {
		return svc, err
	}

	return svc, nil
}

func (r *MatrixoneClusterReconciler) makeHeadlessService(svc *v1.Service, moc *matrixonev1alpha1.MatrixoneCluster, ls map[string]string) (*v1.Service, error) {
	svc.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Service",
	}

	svc.ObjectMeta = metav1.ObjectMeta{
		Name:      moc.Name + "-headless",
		Namespace: moc.Namespace,
		Labels:    ls,
	}

	svc.Spec.Selector = ls
	svc.Spec.ClusterIP = "None"
	svc.Spec.Ports = port

	if err := ctrl.SetControllerReference(moc, svc, r.Scheme); err != nil {
		return svc, err
	}

	return svc, nil
}
