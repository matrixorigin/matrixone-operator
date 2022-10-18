// Copyright 2022 Matrix Origin
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
	"fmt"
	"strings"
	"time"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	createCNSetTimeout = 5 * time.Minute
)

var _ = Describe("MatrixOneCluster test", func() {
	It("Should reconcile the cluster properly", func() {
		By("Create cnset")
		l := &v1alpha1.LogSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: env.Namespace,
				Name:      "cn",
			},
			Spec: v1alpha1.LogSetSpec{
				LogSetBasic: v1alpha1.LogSetBasic{
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
						FileSystem: &v1alpha1.FileSystemProvider{
							Path: "/test",
						},
					},
					StoreFailureTimeout: &metav1.Duration{Duration: 2 * time.Minute},
				},
			},
		}
		d := &v1alpha1.DNSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: env.Namespace,
				Name:      "cn",
			},
			Spec: v1alpha1.DNSetSpec{
				DNSetBasic: v1alpha1.DNSetBasic{
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
			},
			Deps: v1alpha1.DNSetDeps{
				LogSetRef: v1alpha1.LogSetRef{
					LogSet: &v1alpha1.LogSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cn",
							Namespace: env.Namespace,
						},
					},
				},
			},
		}
		c := &v1alpha1.CNSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: env.Namespace,
				Name:      "cn-" + rand.String(6),
			},
			Spec: v1alpha1.CNSetSpec{
				Role: v1alpha1.CNRoleTP,
				CNSetBasic: v1alpha1.CNSetBasic{
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
			},
			Deps: v1alpha1.CNSetDeps{
				LogSetRef: v1alpha1.LogSetRef{
					LogSet: &v1alpha1.LogSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cn",
							Namespace: env.Namespace,
						},
					},
				},
				DNSet: &v1alpha1.DNSet{ObjectMeta: metav1.ObjectMeta{
					Name:      "cn",
					Namespace: env.Namespace,
				}},
			},
		}
		Expect(kubeCli.Create(ctx, l)).To(Succeed())
		Expect(kubeCli.Create(ctx, d)).To(Succeed())
		Expect(kubeCli.Create(ctx, c)).To(Succeed())

		Eventually(func() error {
			if err := kubeCli.Get(ctx, client.ObjectKeyFromObject(c), c); err != nil {
				logger.Errorw("error get cnset status", "cnset", c.Name, "error", err)
				return err
			}
			if !recon.IsReady(&c.Status.ConditionalStatus) {
				logger.Infow("wait cnset ready", "cnset", c.Name)
				return errWait
			}
			return nil
		}, createCNSetTimeout, pollInterval).Should(Succeed())

		By("Teardown cnset")
		Expect(kubeCli.Delete(ctx, c)).To(Succeed())
		Expect(kubeCli.Delete(ctx, d)).To(Succeed())
		Expect(kubeCli.Delete(ctx, l)).To(Succeed())
		Eventually(func() error {
			err := kubeCli.Get(ctx, client.ObjectKeyFromObject(c), c)
			if err == nil {
				logger.Infow("wait cnset teardown", "cnset", c.Name)
				return errWait
			}
			if !apierrors.IsNotFound(err) {
				logger.Errorw("unexpected error when get cnset", "cnset", c, "error", err)
				return err
			}
			podList := &corev1.PodList{}
			err = kubeCli.List(ctx, podList, client.InNamespace(c.Namespace))
			if err != nil {
				logger.Errorw("error list pods", "error", err)
				return err
			}
			for _, pod := range podList.Items {
				if strings.HasPrefix(pod.Name, c.Name) {
					logger.Infow("Pod that belongs to the cnset is not cleaned", "pod", pod.Name)
					return errWait
				}
			}
			return nil
		}, teardownClusterTimeout, pollInterval).Should(Succeed())
	})
})
