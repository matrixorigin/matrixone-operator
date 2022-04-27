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

package components

import (
	"github.com/matrixorigin/matrixone-operator/pkg/apis/matrixone/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func MakeService(svc *corev1.Service, moc *v1alpha1.MatrixoneCluster, ls map[string]string, isHeadless bool) (*corev1.Service, error) {
	svc.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Service",
	}

	if !isHeadless {
		svc.ObjectMeta.Name = moc.Name
		svc.Spec.Type = moc.Spec.ServiceType
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "server",
				Port:       serverPort,
				TargetPort: intstr.FromInt(int(serverPort)),
			},
		}
	} else {
		svc.ObjectMeta.Name = moc.Name + "-headless"
		svc.Spec.ClusterIP = "None"
		svc.Spec.Ports = []corev1.ServicePort{
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

	}

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

	return svc, nil
}

func MakeServiceEmptyObj() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
	}
}

func MakeServiceListEmptyObj() *corev1.ServiceList {
	return &corev1.ServiceList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
	}
}
