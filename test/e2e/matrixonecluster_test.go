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
	"strings"
	"time"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	createClusterTimeout   = 5 * time.Minute
	rollingUpdateTimeout   = 5 * time.Minute
	teardownClusterTimeout = 5 * time.Minute
	pollInterval           = 15 * time.Second
)

var _ = Describe("MatrixOneCluster test", func() {
	It("Should reconcile the cluster properly", func() {
		By("Create cluster")
		mo := &v1alpha1.MatrixOneCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: env.Namespace,
				Name:      "test",
			},
			Spec: v1alpha1.MatrixOneClusterSpec{
				TP: v1alpha1.CNSetBasic{
					PodSet: v1alpha1.PodSet{
						Replicas: 2,
					},
					CacheVolume: &v1alpha1.Volume{
						Size: resource.MustParse("100Mi"),
					},
				},
				DN: v1alpha1.DNSetBasic{
					PodSet: v1alpha1.PodSet{
						Replicas: 1,
					},
					CacheVolume: &v1alpha1.Volume{
						Size: resource.MustParse("100Mi"),
					},
				},
				LogService: v1alpha1.LogSetBasic{
					PodSet: v1alpha1.PodSet{
						Replicas: 3,
					},
					Volume: v1alpha1.Volume{
						Size: resource.MustParse("100Mi"),
					},
					SharedStorage: v1alpha1.SharedStorageProvider{
						S3: &v1alpha1.S3Provider{
							Path: "mo-e2e/mocluster",
						},
					},
					InitialConfig: v1alpha1.InitialConfig{},
				},
				Version:         moVersion,
				ImageRepository: moImageRepo,
			},
		}
		Expect(kubeCli.Create(ctx, mo)).To(Succeed())

		Eventually(func() error {
			if err := kubeCli.Get(ctx, client.ObjectKeyFromObject(mo), mo); err != nil {
				logger.Errorw("error get mo cluster status", "cluster", mo.Name, "error", err)
				return err
			}
			if mo.Status.TP == nil || !recon.IsReady(mo.Status.TP) {
				logger.Infow("wait mo cluster ready", "cluster", mo.Name)
				return errWait
			}
			return nil
		}, createClusterTimeout, pollInterval).Should(Succeed())
		Eventually(func() error {
			podList := &corev1.PodList{}
			if err := kubeCli.List(ctx, podList); err != nil {
				logger.Errorw("error list pods", "cluster", mo.Name, "error", err)
				return err
			}
			var logN, dnN, cnN int32
			for _, pod := range podList.Items {
				switch pod.Labels[common.ComponentLabelKey] {
				case "LogSet":
					logN++
				case "DNSet":
					dnN++
				case "CNSet":
					cnN++
				}
			}
			if logN >= mo.Spec.LogService.Replicas && dnN >= mo.Spec.DN.Replicas && cnN >= mo.Spec.TP.Replicas {
				return nil
			}
			logger.Infow("wait enough pods running", "log pods count", logN, "cn pods count", cnN, "dn pods count", dnN)
			return errWait
		}, createClusterTimeout, pollInterval).Should(Succeed())

		By("Rolling-update cluster config")
		configTemplate := v1alpha1.NewTomlConfig(map[string]interface{}{
			"log": map[string]interface{}{
				"level": "info",
			},
		})
		mo.Spec.LogService.Config = configTemplate.DeepCopy()
		mo.Spec.DN.Config = configTemplate.DeepCopy()
		mo.Spec.TP.Config = configTemplate.DeepCopy()
		Expect(kubeCli.Update(ctx, mo)).To(Succeed())
		verifyConfig := func(pods []corev1.Pod, comp string) error {
			var configMapName string
			for _, pod := range pods {
				if pod.Labels[common.ComponentLabelKey] != comp {
					continue
				}
				configVolume := util.FindFirst(pod.Spec.Volumes, util.WithVolumeName("config"))
				// for all pods of the same component, we verify:
				if configMapName == "" {
					configMapName = configVolume.ConfigMap.Name
				}
				if configMapName != configVolume.ConfigMap.Name {
					return errors.New("rolling-update of pods' configmap do not complete, wait")
				}
			}
			cm := &corev1.ConfigMap{}
			// now that all pods have the same configmap, we verify whether the configmap is the desired new one
			if err := kubeCli.Get(ctx, client.ObjectKey{Namespace: mo.Namespace, Name: configMapName}, cm); err != nil {
				return errors.New("pods are being rolling-updated")
			}
			var foundConfig bool
			for _, data := range cm.Data {
				if strings.Contains(data, "level = \"info\"") {
					foundConfig = true
				}
			}
			if foundConfig {
				return nil
			}
			return errors.New("configmap does not update to date")
		}
		Eventually(func() error {
			podList := &corev1.PodList{}
			if err := kubeCli.List(ctx, podList); err != nil {
				logger.Errorw("error list pods", "cluster", mo.Name, "error", err)
				return err
			}
			var errs error
			for _, comp := range []string{"LogSet", "DNSet", "CNSet"} {
				errs = multierr.Append(errs, verifyConfig(podList.Items, comp))
			}
			if errs != nil {
				logger.Infow("wait for cluster rolling update complete", "error", errs)
				return errs
			}
			return nil
		}, rollingUpdateTimeout, pollInterval).Should(Succeed())

		By("Teardown cluster")
		Expect(kubeCli.Delete(ctx, mo)).To(Succeed())
		Eventually(func() error {
			err := kubeCli.Get(ctx, client.ObjectKeyFromObject(mo), mo)
			if err == nil {
				logger.Infow("wait mo cluster teardown", "cluster", mo.Name)
				return errWait
			}
			if !apierrors.IsNotFound(err) {
				logger.Errorw("unexpected error when get mo cluster", "cluster", mo.Name, "error", err)
				return err
			}
			return nil
		}, teardownClusterTimeout, pollInterval).Should(Succeed(), "cluster should be teardown")
	})
})
