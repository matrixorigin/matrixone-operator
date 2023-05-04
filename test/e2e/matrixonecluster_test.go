// Copyright 2023 Matrix Origin
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
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/test/e2e/sql"
	e2eutil "github.com/matrixorigin/matrixone-operator/test/e2e/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

const (
	createClusterTimeout   = 5 * time.Minute
	rollingUpdateTimeout   = 5 * time.Minute
	teardownClusterTimeout = 10 * time.Minute
	pollInterval           = 15 * time.Second
	portForwardTimeout     = 10 * time.Second
	sqlTestTimeout         = 5 * time.Minute
)

var _ = Describe("MatrixOneCluster test", func() {
	It("Should reconcile the cluster properly", func() {
		By("Create cluster")
		s3TypeMinio := v1alpha1.S3ProviderTypeMinIO
		minioSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: env.Namespace,
				Name:      "minio",
			},
			StringData: map[string]string{
				"AWS_ACCESS_KEY_ID":     "minio",
				"AWS_SECRET_ACCESS_KEY": "minio123",
			},
		}
		Expect(kubeCli.Create(context.TODO(), minioSecret)).To(Succeed())
		mo := &v1alpha1.MatrixOneCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: env.Namespace,
				Name:      "test",
			},
			Spec: v1alpha1.MatrixOneClusterSpec{
				TP: v1alpha1.CNSetSpec{
					PodSet: v1alpha1.PodSet{
						Replicas: 2,
					},
					CacheVolume: &v1alpha1.Volume{
						Size: resource.MustParse("100Mi"),
					},
				},
				DN: v1alpha1.DNSetSpec{
					PodSet: v1alpha1.PodSet{
						// test multiple DN replicas
						Replicas: 1,
					},
					CacheVolume: &v1alpha1.Volume{
						Size: resource.MustParse("100Mi"),
					},
				},
				LogService: v1alpha1.LogSetSpec{
					PodSet: v1alpha1.PodSet{
						Replicas: 3,
					},
					Volume: v1alpha1.Volume{
						Size: resource.MustParse("100Mi"),
					},
					SharedStorage: v1alpha1.SharedStorageProvider{
						S3: &v1alpha1.S3Provider{
							Path:     "matrixone",
							Type:     &s3TypeMinio,
							Endpoint: "http://minio.default:9000",
							SecretRef: &corev1.LocalObjectReference{
								Name: minioSecret.Name,
							},
						},
					},
					InitialConfig: v1alpha1.InitialConfig{},
				},
				Proxy: &v1alpha1.ProxySetSpec{
					PodSet: v1alpha1.PodSet{
						Replicas: 1,
					},
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
		var proxyPod *corev1.Pod
		Eventually(func() error {
			podList := &corev1.PodList{}
			if err := kubeCli.List(ctx, podList); err != nil {
				logger.Errorw("error list pods", "cluster", mo.Name, "error", err)
				return err
			}
			var logN, dnN, cnN, proxyN int32
			for i, pod := range podList.Items {
				if !util.IsPodReady(&pod) {
					continue
				}
				switch pod.Labels[common.ComponentLabelKey] {
				case "LogSet":
					logN++
				case "DNSet":
					dnN++
				case "CNSet":
					cnN++
				case "ProxySet":
					if proxyPod == nil {
						// simply pick the first proxy pod we encounter to perform the following test
						proxyPod = &podList.Items[i]
					}
					proxyN++
				}
			}
			if logN >= mo.Spec.LogService.Replicas && dnN >= mo.Spec.DN.Replicas && cnN >= mo.Spec.TP.Replicas && proxyN >= mo.Spec.Proxy.Replicas {
				return nil
			}
			logger.Infow("wait enough pods running", "log pods count", logN, "cn pods count", cnN, "dn pods count", dnN)
			return errWait
		}, createClusterTimeout, pollInterval).Should(Succeed())

		By("End to end SQL")
		pfh, err := e2eutil.PortForward(restConfig, mo.Namespace, proxyPod.Name, 6001, 6001)
		Expect(err).To(BeNil())
		Expect(pfh.Ready(portForwardTimeout)).To(Succeed(), "port-forward should complete within timeout")
		logger.Info("run SQL smoke test")
		Eventually(func() error {
			err := sql.MySQLDialectSmokeTest("dump:111@tcp(127.0.0.1:6001)/test?timeout=15s")
			if err != nil {
				logger.Infow("error running sql", "error", err)
				return errWait
			}
			return nil
		}, sqlTestTimeout, pollInterval).Should(Succeed(), "SQL smoke test should succeed")
		pfh.Stop()

		By("Rolling-update cluster config")
		configTemplate := v1alpha1.NewTomlConfig(map[string]interface{}{
			"log": map[string]interface{}{
				"level": "info",
			},
		})
		Expect(kubeCli.Get(ctx, client.ObjectKeyFromObject(mo), mo)).To(Succeed())
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
				if pod.Labels[common.MatrixoneClusterLabelKey] != mo.Name {
					continue
				}
				configVolume := util.FindFirst(pod.Spec.Volumes, util.WithVolumeName("config"))
				// for all pods of the same component, we verify:
				if configMapName == "" {
					configMapName = configVolume.ConfigMap.Name
				}
				if configMapName != configVolume.ConfigMap.Name {
					logger.Info("rolling-update of pods' configmap do not complete, wait")
					return errWait
				}
			}
			cm := &corev1.ConfigMap{}
			// now that all pods have the same configmap, we verify whether the configmap is the desired new one
			if err := kubeCli.Get(ctx, client.ObjectKey{Namespace: mo.Namespace, Name: configMapName}, cm); err != nil {
				logger.Info("pods are being rolling-updated")
				return errWait
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
			logger.Info("configmap does not update to date")
			return errWait
		}
		Eventually(func() error {
			podList := &corev1.PodList{}
			if err := kubeCli.List(ctx, podList, client.InNamespace(env.Namespace)); err != nil {
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
			pvcList := &corev1.PersistentVolumeClaimList{}
			err = kubeCli.List(ctx, pvcList, client.InNamespace(mo.Namespace))
			if err != nil {
				logger.Errorw("error list PVCs", "error", err)
				return errWait
			}
			for _, pvc := range pvcList.Items {
				if strings.HasPrefix(pvc.Name, fmt.Sprintf("data-%s", mo.Name)) {
					logger.Infow("pvc is not yet cleaned", "name", pvc.Name)
					return errWait
				}
			}
			return nil
		}, teardownClusterTimeout, pollInterval).Should(Succeed(), "cluster should be teardown")
	})
})
