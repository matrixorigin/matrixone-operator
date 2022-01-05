package controllers

import (
	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	dataPath       string = "/opt/matrixone/store"
	logPath        string = "/opt/matrixone/log"
	ServerPort     int32  = 6001
	addrRaftPort   int32  = 10000
	addrClientPort int32  = 20000
	rpcAddrPort    int32  = 30000
	clientPort     int32  = 40000
	peerPort       int32  = 50000
	raftPort       int32  = 20100
)

func (r *MatrixoneClusterReconciler) makeStatefulset(moc *matrixonev1alpha1.MatrixoneCluster, ls map[string]string) (appsv1.StatefulSet, error) {
	logVolName := "log"
	dataVolName := "data"
	firstNode := moc.Name + "-0"

	ss := appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      moc.Name,
			Namespace: moc.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &moc.Spec.Size,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			PodManagementPolicy: moc.Spec.PodManagementPolicy,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:    ls,
					Namespace: moc.Namespace,
				},
				Spec: corev1.PodSpec{
					// Volumes: []corev1.Volume{
					// 	{
					// 		Name: moc.Spec.PodName.Value,
					// 		VolumeSource: corev1.VolumeSource{
					// 			EmptyDir: &corev1.EmptyDirVolumeSource{},
					// 		},
					// 	},
					// },
					NodeSelector: moc.Spec.NodeSelector,
					Affinity:     moc.Spec.Affinity,
					Tolerations:  moc.Spec.Tolerations,
					Containers: []corev1.Container{
						{
							Name:            moc.Name,
							Image:           moc.Spec.Image,
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								{
									Name:  "FIRST_NODE",
									Value: firstNode,
								},
								moc.Spec.PodName,
								moc.Spec.PodIP,
								moc.Spec.PodNameSpace,
							},
							Resources:      moc.Spec.Resources,
							LivenessProbe:  moc.Spec.LivenessProbe,
							ReadinessProbe: moc.Spec.ReadinessProbe,
							Command:        moc.Spec.Command,
							Ports: []corev1.ContainerPort{
								{
									Name:          "server",
									ContainerPort: ServerPort,
								},
								{
									Name:          "addr-raft",
									ContainerPort: addrRaftPort,
								},
								{
									Name:          "addr-client",
									ContainerPort: addrClientPort,
								},
								{
									Name:          "rpc",
									ContainerPort: rpcAddrPort,
								},
								{
									Name:          "client",
									ContainerPort: clientPort,
								},
								{
									Name:          "peer",
									ContainerPort: peerPort,
								},
								{
									Name:          "raft",
									ContainerPort: raftPort,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      logVolName,
									MountPath: logPath,
								},
								{
									Name:      dataVolName,
									MountPath: dataPath,
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      logVolName,
						Namespace: moc.Namespace,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							"ReadWriteOnce",
						},
						Resources:        moc.Spec.LogVolResource,
						StorageClassName: &moc.Spec.StorageClass,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dataVolName,
						Namespace: moc.Namespace,
						Labels:    ls,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							"ReadWriteOnce",
						},
						Resources:        moc.Spec.DataVolResource,
						StorageClassName: &moc.Spec.StorageClass,
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(moc, &ss, r.Scheme); err != nil {
		return ss, err
	}

	return ss, nil
}
