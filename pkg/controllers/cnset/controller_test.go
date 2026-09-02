// Copyright 2025-2026 Matrix Origin
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

package cnset

import (
	"context"
	"strconv"
	"testing"

	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"

	"github.com/golang/mock/gomock"
	"github.com/matrixorigin/controller-runtime/pkg/fake"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/utils"
	. "github.com/onsi/gomega"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func baseCNSetForMetricSvcTest() *v1alpha1.CNSet {
	return &v1alpha1.CNSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       "CNSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test",
			UID:       "test-uid",
		},
		Spec: v1alpha1.CNSetSpec{
			PodSet: v1alpha1.PodSet{
				MainContainer: v1alpha1.MainContainer{
					Image: "test:latest",
				},
				Replicas: 1,
			},
		},
	}
}

func enableCNPromServiceDiscovery(cn *v1alpha1.CNSet) {
	cn.Spec.ExportToPrometheus = pointer.Bool(true)
	scheme := v1alpha1.PromDiscoverySchemeService
	cn.Spec.PromDiscoveryScheme = &scheme
}

// Test_syncMetricService is a regression test for issue #600: CNSet must reconcile a
// dedicated metric Service so ServiceMonitor can match on port name "metric".
func Test_syncMetricService(t *testing.T) {
	s := newScheme()
	labels := common.SubResourceLabels(baseCNSetForMetricSvcTest())
	labels[common.ComponentLabelKey] = "CNSet"

	tests := []struct {
		name   string
		cnset  *v1alpha1.CNSet
		client client.Client
		setup  func(cn *v1alpha1.CNSet)
		expect func(g *WithT, cn *v1alpha1.CNSet, cli client.Client, err error)
	}{
		{
			name:  "creates metric service with prom annotations when export enabled",
			cnset: baseCNSetForMetricSvcTest(),
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).Build(),
			},
			setup: enableCNPromServiceDiscovery,
			expect: func(g *WithT, cn *v1alpha1.CNSet, cli client.Client, err error) {
				g.Expect(err).To(BeNil())

				svc := &corev1.Service{}
				g.Expect(cli.Get(context.Background(), client.ObjectKey{
					Namespace: cn.Namespace,
					Name:      metricSvcName(cn),
				}, svc)).To(Succeed())

				g.Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
				g.Expect(svc.Spec.Ports).To(HaveLen(1))
				g.Expect(svc.Spec.Ports[0].Name).To(Equal("metric"))
				g.Expect(svc.Spec.Ports[0].Port).To(Equal(int32(common.MetricsPort)))
				g.Expect(svc.Labels).To(Equal(labels))
				g.Expect(svc.Spec.Selector).To(Equal(labels))
				g.Expect(svc.Annotations[common.PrometheusScrapeAnno]).To(Equal("true"))
				g.Expect(svc.Annotations[common.PrometheusPortAnno]).To(Equal(strconv.Itoa(common.MetricsPort)))
			},
		},
		{
			name:  "creates metric service without prom annotations when export disabled",
			cnset: baseCNSetForMetricSvcTest(),
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).Build(),
			},
			expect: func(g *WithT, cn *v1alpha1.CNSet, cli client.Client, err error) {
				g.Expect(err).To(BeNil())

				svc := &corev1.Service{}
				g.Expect(cli.Get(context.Background(), client.ObjectKey{
					Namespace: cn.Namespace,
					Name:      metricSvcName(cn),
				}, svc)).To(Succeed())

				g.Expect(svc.Spec.Ports).To(HaveLen(1))
				g.Expect(svc.Spec.Ports[0].Name).To(Equal("metric"))
				g.Expect(svc.Annotations).NotTo(HaveKey(common.PrometheusScrapeAnno))
				g.Expect(svc.Annotations).NotTo(HaveKey(common.PrometheusPortAnno))
			},
		},
		{
			name:  "repairs drift while preserving user metadata",
			cnset: baseCNSetForMetricSvcTest(),
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).WithObjects(
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Namespace:   "default",
							Name:        metricSvcName(baseCNSetForMetricSvcTest()),
							Labels:      map[string]string{"drifted": "true"},
							Annotations: map[string]string{"user.example.com/keep": "yes"},
						},
						Spec: corev1.ServiceSpec{
							Type:     corev1.ServiceTypeNodePort,
							Selector: map[string]string{"wrong": "selector"},
							Ports: []corev1.ServicePort{{
								Name: "wrong",
								Port: 1234,
							}},
						},
					},
				).Build(),
			},
			setup: enableCNPromServiceDiscovery,
			expect: func(g *WithT, cn *v1alpha1.CNSet, cli client.Client, err error) {
				g.Expect(err).To(BeNil())
				svc := &corev1.Service{}
				g.Expect(cli.Get(context.Background(), client.ObjectKeyFromObject(&corev1.Service{ObjectMeta: metav1.ObjectMeta{
					Namespace: cn.Namespace,
					Name:      metricSvcName(cn),
				}}), svc)).To(Succeed())
				g.Expect(svc.Labels).To(HaveKeyWithValue("drifted", "true"))
				for key, value := range labels {
					g.Expect(svc.Labels).To(HaveKeyWithValue(key, value))
				}
				g.Expect(svc.Spec.Selector).To(Equal(labels))
				g.Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
				g.Expect(svc.Spec.Ports).To(Equal([]corev1.ServicePort{{Name: "metric", Port: int32(common.MetricsPort)}}))
				g.Expect(svc.Annotations).To(HaveKeyWithValue("user.example.com/keep", "yes"))
				g.Expect(svc.Annotations).To(HaveKeyWithValue(common.PrometheusScrapeAnno, "true"))
				g.Expect(metav1.IsControlledBy(svc, cn)).To(BeTrue())
			},
		},
		{
			name:  "removes stale prom annotations when export disabled",
			cnset: baseCNSetForMetricSvcTest(),
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).WithObjects(
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Name:      metricSvcName(baseCNSetForMetricSvcTest()),
							Labels:    labels,
							Annotations: map[string]string{
								common.PrometheusScrapeAnno: "true",
								common.PrometheusPortAnno:   strconv.Itoa(common.MetricsPort),
							},
						},
						Spec: corev1.ServiceSpec{
							Type:     corev1.ServiceTypeClusterIP,
							Selector: labels,
							Ports: []corev1.ServicePort{{
								Name: "metric",
								Port: int32(common.MetricsPort),
							}},
						},
					},
				).Build(),
			},
			expect: func(g *WithT, cn *v1alpha1.CNSet, cli client.Client, err error) {
				g.Expect(err).To(BeNil())

				svc := &corev1.Service{}
				g.Expect(cli.Get(context.Background(), client.ObjectKey{
					Namespace: cn.Namespace,
					Name:      metricSvcName(cn),
				}, svc)).To(Succeed())
				g.Expect(svc.Annotations).NotTo(HaveKey(common.PrometheusScrapeAnno))
				g.Expect(svc.Annotations).NotTo(HaveKey(common.PrometheusPortAnno))
			},
		},
		{
			name:  "does not mutate service controlled by another owner",
			cnset: baseCNSetForMetricSvcTest(),
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).WithObjects(
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "default",
							Name:      metricSvcName(baseCNSetForMetricSvcTest()),
							Labels:    map[string]string{"foreign": "owner"},
							OwnerReferences: []metav1.OwnerReference{{
								APIVersion: "example.com/v1",
								Kind:       "Example",
								Name:       "foreign",
								UID:        "foreign-uid",
								Controller: pointer.Bool(true),
							}},
						},
						Spec: corev1.ServiceSpec{
							Type:     corev1.ServiceTypeNodePort,
							Selector: map[string]string{"foreign": "selector"},
							Ports:    []corev1.ServicePort{{Name: "foreign", Port: 1234}},
						},
					},
				).Build(),
			},
			expect: func(g *WithT, cn *v1alpha1.CNSet, cli client.Client, err error) {
				g.Expect(err).To(BeNil())
				svc := &corev1.Service{}
				g.Expect(cli.Get(context.Background(), client.ObjectKey{Namespace: cn.Namespace, Name: metricSvcName(cn)}, svc)).To(Succeed())
				g.Expect(svc.Labels).To(Equal(map[string]string{"foreign": "owner"}))
				g.Expect(svc.Spec.Selector).To(Equal(map[string]string{"foreign": "selector"}))
				g.Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			cn := tt.cnset.DeepCopy()
			if tt.setup != nil {
				tt.setup(cn)
			}

			mockCtrl := gomock.NewController(t)
			eventEmitter := fake.NewMockEventEmitter(mockCtrl)
			ctx := fake.NewContext(cn, tt.client, eventEmitter)

			err := (&Actor{}).syncMetricService(ctx, common.SubResourceLabels(cn))
			tt.expect(g, cn, tt.client, err)
		})
	}
}

func TestMetricServiceSelectorMatchesCNPodLabels(t *testing.T) {
	for _, tc := range []struct {
		name         string
		withTypeMeta bool
	}{
		{name: "without TypeMeta", withTypeMeta: false},
		{name: "with TypeMeta", withTypeMeta: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			cn := baseCNSetForMetricSvcTest()
			if !tc.withTypeMeta {
				cn.TypeMeta = metav1.TypeMeta{}
			}
			cli := &fake.Client{Client: fake.KubeClientBuilder().WithScheme(newScheme()).Build()}
			ctx := fake.NewContext(cn, cli, fake.NewMockEventEmitter(gomock.NewController(t)))

			cnPods := buildCNSet(cn, &corev1.Service{}).Spec.Template.Labels
			g.Expect((&Actor{}).syncMetricService(ctx, cnPods)).To(Succeed())
			svc := &corev1.Service{}
			g.Expect(cli.Get(context.Background(), client.ObjectKey{
				Namespace: cn.Namespace,
				Name:      metricSvcName(cn),
			}, svc)).To(Succeed())

			g.Expect(svc.Spec.Selector).To(Equal(cnPods))
			g.Expect(svc.Labels).To(HaveKeyWithValue(common.ComponentLabelKey, "CNSet"))
		})
	}
}

func TestRequestsForLogSetStatefulSet(t *testing.T) {
	s := newScheme()
	logSet := &v1alpha1.LogSet{ObjectMeta: metav1.ObjectMeta{Namespace: "provider", Name: "log", UID: "log-uid"}}
	matching := &v1alpha1.CNSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "consumer", Name: "matching"},
		Deps:       v1alpha1.CNSetDeps{LogSetRef: logSet.AsDependency()},
	}
	unrelated := &v1alpha1.CNSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "consumer", Name: "unrelated"},
		Deps: v1alpha1.CNSetDeps{LogSetRef: v1alpha1.LogSetRef{LogSet: &v1alpha1.LogSet{
			ObjectMeta: metav1.ObjectMeta{Namespace: "other", Name: "log"},
		}}},
	}
	cli := fake.KubeClientBuilder().WithScheme(s).WithObjects(matching, unrelated).Build()
	sts := &kruisev1.StatefulSet{ObjectMeta: metav1.ObjectMeta{
		Namespace: "provider",
		Name:      "log-log",
		OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(logSet,
			v1alpha1.GroupVersion.WithKind("LogSet"))},
	}}

	requests := requestsForLogSetStatefulSet(cli)(context.Background(), sts)
	g := NewGomegaWithT(t)
	g.Expect(requests).To(Equal([]reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(matching)}}))
}

func TestCNSetActor_Observe(t *testing.T) {
	s := newScheme()
	tpl := &v1alpha1.CNSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test",
		},
		Spec: v1alpha1.CNSetSpec{
			PodSet: v1alpha1.PodSet{
				MainContainer: v1alpha1.MainContainer{
					Image: "test:latest",
				},
				Replicas: 1,
			},
			ConfigThatChangeCNSpec: v1alpha1.ConfigThatChangeCNSpec{
				CacheVolume: &v1alpha1.Volume{
					Size: resource.MustParse("10Gi"),
				},
			},
		},
	}
	tplNoVolume := &v1alpha1.CNSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test",
		},
		Spec: v1alpha1.CNSetSpec{
			PodSet: v1alpha1.PodSet{
				MainContainer: v1alpha1.MainContainer{
					Image: "test:latest",
				},
				Replicas: 3,
			},
		},
	}
	labels := common.SubResourceLabels(tpl)
	n := setName(tpl)
	svc := svcName(tpl)
	tests := []struct {
		name   string
		cnset  *v1alpha1.CNSet
		client client.Client
		expect func(g *WithT, action recon.Action[*v1alpha1.CNSet], cli client.Client, err error)
	}{
		{
			name:  "create when resource not exist",
			cnset: tpl,
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).Build(),
			},
			expect: func(g *WithT, action recon.Action[*v1alpha1.CNSet], _ client.Client, err error) {
				g.Expect(err).To(BeNil())
				g.Expect(action.String()).To(ContainSubstring("Create"))
			},
		},
		{
			name:  "create when resource not exist and no cache volume config",
			cnset: tplNoVolume,
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).Build(),
			},
			expect: func(g *WithT, action recon.Action[*v1alpha1.CNSet], _ client.Client, err error) {
				g.Expect(err).To(BeNil())
				g.Expect(action.String()).To(ContainSubstring("Create"))
			},
		},
		{
			name:  "update with volumeClaim",
			cnset: tpl,
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).WithObjects(
					&kruisev1alpha1.CloneSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      n,
							Namespace: "default",
						},
						Spec: kruisev1alpha1.CloneSetSpec{
							Replicas: pointer.Int32(1),
							Template: corev1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: labels,
								},
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "main",
											Image: "test:latest",
										},
									},
								},
							},
							VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
								{
									ObjectMeta: metav1.ObjectMeta{
										Labels: labels,
									},
									Spec: corev1.PersistentVolumeClaimSpec{
										AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
										Resources: corev1.ResourceRequirements{
											Requests: map[corev1.ResourceName]resource.Quantity{
												corev1.ResourceStorage: resource.MustParse("10Gi"),
											},
										},
									},
								},
							},
						},
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      svc,
							Namespace: "default",
							Labels:    labels,
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
						},
					},
					// the LogSet's own StatefulSet must exist by the time CN builds its
					// ConfigMap (fetchLogSetReservedOrdinals requires it, see #596).
					&kruisev1.StatefulSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-log",
							Namespace: "default",
						},
					},
				).Build(),
			},
			expect: func(g *WithT, action recon.Action[*v1alpha1.CNSet], _ client.Client, err error) {
				g.Expect(err).To(BeNil())
				g.Expect(action.String()).To(ContainSubstring("Update"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			r := &Actor{}
			mockCtrl := gomock.NewController(t)
			eventEmitter := fake.NewMockEventEmitter(mockCtrl)
			ctx := fake.NewContext(tt.cnset, tt.client, eventEmitter)
			ctx.Dep = tt.cnset.DeepCopy()
			ctx.Dep.Deps.LogSet = &v1alpha1.LogSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: v1alpha1.LogSetSpec{
					SharedStorage: v1alpha1.SharedStorageProvider{
						S3: &v1alpha1.S3Provider{
							Path: "bucket/dir",
						},
					},
				},
				Status: v1alpha1.LogSetStatus{
					Discovery: &v1alpha1.LogSetDiscovery{
						Port:    6001,
						Address: "test",
					},
				},
			}
			action, err := r.Observe(ctx)
			tt.expect(g, action, tt.client, err)
		})
	}
}

func TestCNSetVolumeMount(t *testing.T) {
	s := newScheme()

	tests := []struct {
		name   string
		cs     *kruisev1alpha1.CloneSet
		cnset  *v1alpha1.CNSet
		sp     v1alpha1.SharedStorageProvider
		client client.Client
	}{
		{
			name: "test volume mount",
			cnset: &v1alpha1.CNSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: v1alpha1.CNSetSpec{
					PodSet: v1alpha1.PodSet{
						MainContainer: v1alpha1.MainContainer{
							Image: "test:latest",
						},
						Replicas: 3,
					},
				},
			},
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).Build(),
			},
			sp: v1alpha1.SharedStorageProvider{},
			cs: &kruisev1alpha1.CloneSet{},
		},
		{
			name: "test volume mount with cache volume",
			cnset: &v1alpha1.CNSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: v1alpha1.CNSetSpec{
					PodSet: v1alpha1.PodSet{
						MainContainer: v1alpha1.MainContainer{
							Image: "test:latest",
						},
						Replicas: 3,
					},
					ConfigThatChangeCNSpec: v1alpha1.ConfigThatChangeCNSpec{
						CacheVolume: &v1alpha1.Volume{
							Size: resource.MustParse("10Gi"),
						},
					},
				},
			},
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).Build(),
			},
			sp: v1alpha1.SharedStorageProvider{},
			cs: &kruisev1alpha1.CloneSet{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			eventEmitter := fake.NewMockEventEmitter(mockCtrl)
			ctx := fake.NewContext(tt.cnset, tt.client, eventEmitter)
			ctx.Dep = tt.cnset.DeepCopy()
			ctx.Dep.Deps.LogSet = &v1alpha1.LogSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: v1alpha1.LogSetSpec{
					SharedStorage: v1alpha1.SharedStorageProvider{
						S3: &v1alpha1.S3Provider{
							Path: "bucket/dir",
						},
					},
				},
				Status: v1alpha1.LogSetStatus{
					Discovery: &v1alpha1.LogSetDiscovery{
						Port:    6001,
						Address: "test",
					},
				},
			}
			syncPodSpec(tt.cnset, tt.cs, tt.sp)

			if tt.cnset.Spec.CacheVolume == nil {
				// if cacheVolume not set, volumeClaimTemplates should be 0
				// dataVolumeMount should not be created.
				if !utils.CheckVolumeClaimTemplate(common.DataVolume, tt.cs.Spec.VolumeClaimTemplates) {
					if utils.CheckVolumeMount(common.DataVolume, tt.cs.Spec.Template.Spec.Containers[0].VolumeMounts) {
						t.Error("mo data volume create error")
					}
				} else {
					t.Error("should not have a persistent volume for cache when cacheVolume is not set")
				}
			}
		})
	}
}

// Test_fetchLogSetReservedOrdinals is a regression test for issue #596: previously any
// error reading the LogSet StatefulSet (including "not found") was swallowed and treated
// as "no ordinal holes", which could cause service-addresses to be regenerated with a dead
// ordinal on a transient read failure. Errors must now propagate so reconcile retries.
func Test_fetchLogSetReservedOrdinals(t *testing.T) {
	s := newScheme()
	cn := &v1alpha1.CNSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test"},
	}
	ls := &v1alpha1.LogSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test"},
	}

	tests := []struct {
		name         string
		client       client.Client
		ls           *v1alpha1.LogSet
		wantOrdinals []int
		wantErr      bool
	}{
		{
			name: "sts exists with reserved ordinals",
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).WithObjects(
					&kruisev1.StatefulSet{
						ObjectMeta: metav1.ObjectMeta{Name: "test-log", Namespace: "default"},
						Spec:       kruisev1.StatefulSetSpec{ReserveOrdinals: []int{1}},
					},
				).Build(),
			},
			ls:           ls,
			wantOrdinals: []int{1},
		},
		{
			name: "sts not found propagates error instead of falling back silently",
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).Build(),
			},
			ls:      ls,
			wantErr: true,
		},
		{
			name: "nil logset returns no ordinals and no error",
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).Build(),
			},
			ls: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			mockCtrl := gomock.NewController(t)
			eventEmitter := fake.NewMockEventEmitter(mockCtrl)
			ctx := fake.NewContext(cn, tt.client, eventEmitter)

			got, err := fetchLogSetReservedOrdinals(ctx, tt.ls)
			if tt.wantErr {
				g.Expect(err).NotTo(BeNil())
			} else {
				g.Expect(err).To(BeNil())
				g.Expect(got).To(Equal(tt.wantOrdinals))
			}
		})
	}
}

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(kruisev1.AddToScheme(scheme))
	utilruntime.Must(kruisev1alpha1.AddToScheme(scheme))

	return scheme
}
