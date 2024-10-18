// Copyright 2024 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package logset

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/openkruise/kruise-api/apps/pub"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var lsMeta = metav1.ObjectMeta{
	Name:      "test",
	Namespace: "default",
}

func Test_syncPodSpec(t *testing.T) {
	type args struct {
		ls   *v1alpha1.LogSet
		spec *corev1.PodSpec
	}
	resource := corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU: resource.MustParse("1"),
		},
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU: resource.MustParse("1"),
		},
	}
	tests := []struct {
		name string
		args args
		want *corev1.PodSpec
	}{{
		name: "basic",
		args: args{
			ls: &v1alpha1.LogSet{
				TypeMeta: metav1.TypeMeta{
					Kind: "LogSet",
				},
				ObjectMeta: lsMeta,
				Spec: v1alpha1.LogSetSpec{
					PodSet: v1alpha1.PodSet{
						MainContainer: v1alpha1.MainContainer{
							Image:     "test:latest",
							Resources: resource,
						},
						Replicas: 3,
						NodeSelector: map[string]string{
							"arch": "arm64",
						},
						TopologyEvenSpread: []string{"zone"},
						Config: &v1alpha1.TomlConfig{
							MP: map[string]interface{}{
								"log-level": "debug",
							},
						},
					},
					SharedStorage: v1alpha1.SharedStorageProvider{
						S3: &v1alpha1.S3Provider{
							Path: "test/my-bucket",
						},
					},
					InitialConfig: v1alpha1.InitialConfig{
						LogShards:        pointer.Int(1),
						DNShards:         pointer.Int(1),
						LogShardReplicas: pointer.Int(3),
					},
				},
			},
			spec: &corev1.PodSpec{},
		},
		want: &corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:      "main",
				Image:     "test:latest",
				Resources: resource,
				Command:   []string{"/bin/sh", "/etc/logservice/start.sh"},
				VolumeMounts: []corev1.VolumeMount{
					{Name: common.DataVolume, MountPath: common.DataPath},
					{Name: "bootstrap", ReadOnly: true, MountPath: "/etc/bootstrap"},
					{Name: "config", ReadOnly: true, MountPath: "/etc/logservice"},
					{Name: "gossip", ReadOnly: true, MountPath: "/etc/gossip"},
				},
				Env: []corev1.EnvVar{{
					Name: "POD_NAME",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "metadata.name",
						},
					},
				}, {
					Name: "NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "metadata.namespace",
						},
					},
				}, {
					Name: "POD_IP",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "status.podIP",
						},
					},
				}, {
					Name:  "HEADLESS_SERVICE_NAME",
					Value: "test-log-headless",
				}, {
					Name:  "AWS_REGION",
					Value: "us-west-2",
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "bootstrap",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "test-log-bootstrap"},
						DefaultMode:          pointer.Int32(0644),
					},
				},
			}, {
				Name: "gossip",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "test-log-gossip"},
						DefaultMode:          pointer.Int32(0644),
					},
				},
			}},
			ReadinessGates: []corev1.PodReadinessGate{{
				ConditionType: pub.InPlaceUpdateReady,
			}},
			NodeSelector: map[string]string{
				"arch": "arm64",
			},
			TopologySpreadConstraints: []corev1.TopologySpreadConstraint{{
				MaxSkew:           1,
				TopologyKey:       "zone",
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"matrixorigin.io/component": "LogSet",
						"matrixorigin.io/instance":  "test",
						"matrixorigin.io/namespace": "default",
					},
				},
			}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podSpec := tt.args.spec.DeepCopy()
			syncPodSpec(tt.args.ls, podSpec)
			if diff := cmp.Diff(podSpec, tt.want); diff != "" {
				t.Errorf("syncPodSpec(...): -want spec, +got: spec\n%s", diff)
			}
		})
	}
}

func Test_buildHeadlessSvc(t *testing.T) {
	type args struct {
		ls *v1alpha1.LogSet
	}
	labels := common.SubResourceLabels(&v1alpha1.LogSet{ObjectMeta: lsMeta})
	tests := []struct {
		name string
		args args
		want *corev1.Service
	}{{
		name: "basic",
		args: args{
			ls: &v1alpha1.LogSet{
				ObjectMeta: lsMeta,
			},
		},
		want: &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-log-headless",
				Namespace:   "default",
				Labels:      labels,
				Annotations: map[string]string{},
			},
			Spec: corev1.ServiceSpec{
				ClusterIP:                corev1.ClusterIPNone,
				Selector:                 labels,
				PublishNotReadyAddresses: true,
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(buildHeadlessSvc(tt.args.ls), tt.want); diff != "" {
				t.Errorf("buildHeadlessSvc(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func Test_buildStatefulSet(t *testing.T) {
	type args struct {
		ls          *v1alpha1.LogSet
		headlessSvc *corev1.Service
	}
	labels := common.SubResourceLabels(&v1alpha1.LogSet{ObjectMeta: lsMeta})
	tests := []struct {
		name string
		args args
		want *kruisev1.StatefulSet
	}{{
		name: "basic",
		args: args{
			ls: &v1alpha1.LogSet{
				ObjectMeta: lsMeta,
			},
			headlessSvc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-svc",
				},
			},
		},
		want: &kruisev1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-log",
				Namespace: "default",
				Labels:    labels,
			},
			Spec: kruisev1.StatefulSetSpec{
				ServiceName: "test-svc",
				UpdateStrategy: kruisev1.StatefulSetUpdateStrategy{
					Type: appsv1.RollingUpdateStatefulSetStrategyType,
					RollingUpdate: &kruisev1.RollingUpdateStatefulSetStrategy{
						PodUpdatePolicy: kruisev1.InPlaceIfPossiblePodUpdateStrategyType,
					},
				},
				PodManagementPolicy: appsv1.ParallelPodManagement,
				Selector: &metav1.LabelSelector{
					MatchLabels: labels,
				},
				PersistentVolumeClaimRetentionPolicy: &kruisev1.StatefulSetPersistentVolumeClaimRetentionPolicy{
					WhenDeleted: kruisev1.DeletePersistentVolumeClaimRetentionPolicyType,
					WhenScaled:  kruisev1.DeletePersistentVolumeClaimRetentionPolicyType,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels:      labels,
						Annotations: map[string]string{},
					},
				},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(buildStatefulSet(tt.args.ls, tt.args.headlessSvc), tt.want); diff != "" {
				t.Errorf("buildStatefulSet(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func Test_syncPersistentVolumeClaim(t *testing.T) {
	type args struct {
		ls  *v1alpha1.LogSet
		sts *kruisev1.StatefulSet
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syncPersistentVolumeClaim(tt.args.ls, tt.args.sts)
		})
	}
}
