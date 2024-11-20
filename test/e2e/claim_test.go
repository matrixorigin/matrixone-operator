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

package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	waitPoolTimeout = 10 * time.Minute
	bindTimeout     = 2 * time.Minute
	migrateTimeout  = 5 * time.Minute
)

var _ = Describe("CNClaim and CNPool test", func() {
	It("Should reconcile CNPool  properly", func() {
		By("Create base set")
		ns := env.PoolNamespace
		s3TypeMinio := v1alpha1.S3ProviderTypeMinIO
		minioSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      "minio",
			},
			StringData: map[string]string{
				"AWS_ACCESS_KEY_ID":     "minio",
				"AWS_SECRET_ACCESS_KEY": "minio123",
			},
		}
		Expect(kubeCli.Create(context.TODO(), minioSecret)).To(Succeed())
		l := &v1alpha1.LogSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      "pool",
			},
			Spec: v1alpha1.LogSetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 3,
					MainContainer: v1alpha1.MainContainer{
						Image: fmt.Sprintf("%s:%s", moImageRepo, moVersion),
					},
				},
				Volume: v1alpha1.Volume{
					Size: resource.MustParse("100Mi"),
				},
				SharedStorage: v1alpha1.SharedStorageProvider{
					S3: &v1alpha1.S3Provider{
						Path:     "matrixone/pool",
						Type:     &s3TypeMinio,
						Endpoint: "http://minio.default:9000",
						SecretRef: &corev1.LocalObjectReference{
							Name: minioSecret.Name,
						},
					},
				},
			},
		}
		d := &v1alpha1.DNSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      "pool",
			},
			Spec: v1alpha1.DNSetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 1,
					MainContainer: v1alpha1.MainContainer{
						Image: fmt.Sprintf("%s:%s", moImageRepo, moVersion),
					},
				},
				CacheVolume: &v1alpha1.Volume{
					Size: resource.MustParse("100Mi"),
				},
			},
			Deps: v1alpha1.DNSetDeps{
				LogSetRef: v1alpha1.LogSetRef{
					LogSet: &v1alpha1.LogSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pool",
							Namespace: ns,
						},
					},
				},
			},
		}
		pool := &v1alpha1.CNPool{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      "pool",
			},
			Spec: v1alpha1.CNPoolSpec{
				Template: v1alpha1.CNSetSpec{
					PodSet: v1alpha1.PodSet{
						MainContainer: v1alpha1.MainContainer{
							Image: fmt.Sprintf("%s:%s", moImageRepo, moVersion),
						},
						OperatorVersion: pointer.String(v1alpha1.LatestOpVersion.String()),
					},
				},
				Deps: v1alpha1.CNSetDeps{
					LogSetRef: v1alpha1.LogSetRef{
						LogSet: &v1alpha1.LogSet{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pool",
								Namespace: ns,
							},
						},
					},
					DNSet: &v1alpha1.DNSet{ObjectMeta: metav1.ObjectMeta{
						Name:      "pool",
						Namespace: ns,
					}},
				},
				Strategy: v1alpha1.PoolStrategy{
					ScaleStrategy: v1alpha1.PoolScaleStrategy{
						MaxIdle: 2,
					},
				},
			},
		}
		proxy := &v1alpha1.ProxySet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "proxy",
				Namespace: ns,
			},
			Spec: v1alpha1.ProxySetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 1,
					MainContainer: v1alpha1.MainContainer{
						Image: fmt.Sprintf("%s:%s", moImageRepo, moVersion),
					},
				},
			},
			Deps: v1alpha1.ProxySetDeps{
				LogSetRef: v1alpha1.LogSetRef{
					LogSet: &v1alpha1.LogSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pool",
							Namespace: ns,
						},
					},
				},
			},
		}
		Expect(kubeCli.Create(ctx, l)).To(Succeed())
		Expect(kubeCli.Create(ctx, d)).To(Succeed())
		Expect(kubeCli.Create(ctx, pool)).To(Succeed())
		Expect(kubeCli.Create(ctx, proxy)).To(Succeed())

		podList := &corev1.PodList{}
		Eventually(func() error {
			if err := kubeCli.List(ctx, podList,
				client.MatchingLabels{v1alpha1.PoolNameLabel: pool.Name},
				client.InNamespace(pool.Namespace),
			); err != nil {
				logger.Errorw("error list Pool CNs", "CN pool", pool.Name, "error", err)
				return err
			}
			if len(podList.Items) < 2 {
				logger.Infow("wait CN pool ready", "CN pool", pool.Name)
				return errWait
			}
			if !util.IsPodReady(&podList.Items[0]) || !util.IsPodReady(&podList.Items[1]) {
				logger.Infow("wait CN pool ready", "CN pool", pool.Name)
				return errWait
			}
			return nil
		}, waitPoolTimeout, pollInterval).Should(Succeed())

		claim := &v1alpha1.CNClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      "test",
			},
			Spec: v1alpha1.CNClaimSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						v1alpha1.PoolNameLabel: pool.Name,
					},
				},
				CNLabels: []v1alpha1.CNLabel{{
					Key:    "account",
					Values: []string{"sys"},
				}},
			},
		}
		Expect(kubeCli.Create(ctx, claim)).To(Succeed())

		Eventually(func() error {
			if err := kubeCli.Get(ctx, client.ObjectKeyFromObject(claim), claim); err != nil {
				return err
			}
			if claim.Status.Phase != v1alpha1.CNClaimPhaseBound {
				logger.Infow("wait Claim bind", "CN pool", pool.Name)
				return errWait
			}
			return nil
		}, bindTimeout, pollInterval).Should(Succeed())

		By("migrate pod under claim")
		var target *corev1.Pod
		for i := range podList.Items {
			pod := &podList.Items[i]
			if pod.Name != claim.Spec.PodName {
				target = pod
				break
			}
		}
		Expect(target).NotTo(BeNil(), "should find an idle pod to migrate to")
		_, err := controllerutil.CreateOrPatch(ctx, kubeCli, claim, func() error {
			sourcePod := claim.Spec.ClaimPodRef
			claim.Spec.SourcePod = &sourcePod
			claim.Spec.ClaimPodRef = v1alpha1.ClaimPodRef{
				PodName:  target.Name,
				NodeName: target.Spec.NodeName,
			}
			return nil
		})
		Expect(err).To(Succeed())

		Eventually(func() error {
			if err := kubeCli.Get(ctx, client.ObjectKeyFromObject(claim), claim); err != nil {
				return err
			}
			if claim.Status.Store.PodName == target.Name {
				return nil
			}
			logger.Infow("wait migrate complete", "claim", claim.Name)
			return errWait
		}, migrateTimeout, pollInterval).Should(Succeed())
	})
})
