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
	"fmt"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	e2eutil "github.com/matrixorigin/matrixone-operator/test/e2e/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

const (
	waitBucketStatusTimeout = time.Minute * 5
)

var _ = Describe("Matrix BucketClaim test", func() {

	AfterEach(func() {
		// remove bucket finalizer, in case of failed test case block whole test
		buckets := &v1alpha1.BucketClaimList{}
		err := kubeCli.List(ctx, buckets, client.InNamespace(env.Namespace))
		Expect(err).Should(BeNil())
		for _, b := range buckets.Items {
			if err = reclaimBucket(&b); err != nil {
				logger.Infof("fail deleting bucket, %v, %s", client.ObjectKeyFromObject(&b), err.Error())
			}
		}
	})

	It("Should bucket been released use default retain policy(Retain)", func() {
		By("create logset cluster with minio provider")
		minioSecret := e2eutil.MinioSecret(env.Namespace)
		Expect(kubeCli.Create(ctx, minioSecret)).To(Succeed())

		minioProvider := e2eutil.MinioShareStorage(minioSecret.Name, "minio-bucket/bucket-test")
		ls := e2eutil.NewLogSetTpl(env.Namespace, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		ls.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, ls)).To(Succeed())

		var bucket *v1alpha1.BucketClaim
		var err error
		Eventually(func() error {
			bucket, err = v1alpha1.ClaimedBucket(kubeCli, ls)
			if err != nil || bucket == nil {
				return fmt.Errorf("wait bucket creating for logset %v, %v", client.ObjectKeyFromObject(ls), err)
			}
			expectedStatus := v1alpha1.BucketClaimStatus{
				BindTo: v1alpha1.BucketBindToMark(ls),
				State:  v1alpha1.StatusInUse,
			}
			if !reflect.DeepEqual(expectedStatus, bucket.Status) {
				return fmt.Errorf("bucket status is not inuse, current %v", bucket.Status)
			}
			return nil
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("tear down logset cluster")
		Expect(kubeCli.Delete(ctx, ls)).To(Succeed())
		Eventually(func() error {
			return waitLogSetDeleted(ls)
		}, teardownClusterTimeout, pollInterval).Should(Succeed())

		By("bucket should in released state")
		Eventually(func() error {
			if err := kubeCli.Get(ctx, client.ObjectKeyFromObject(bucket), bucket); err != nil {
				return err
			}
			if bucket.DeletionTimestamp != nil {
				return fmt.Errorf("bucket should not be deleted in default retain policy")
			}
			expectedStatus := v1alpha1.BucketClaimStatus{
				BindTo: "",
				State:  v1alpha1.StatusReleased,
			}
			if !reflect.DeepEqual(expectedStatus, bucket.Status) {
				return fmt.Errorf("bucket status is not released, current %v", bucket.Status)
			}
			return nil
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("reclaim bucket")
		Expect(reclaimBucket(bucket)).Should(BeNil())
		Eventually(func() error {
			err := kubeCli.Get(ctx, client.ObjectKeyFromObject(bucket), bucket)
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("bucket should be deleted")
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())
	})

	It("Should bucket been deleted use delete retain policy", func() {
		//By("port forward minio")
		//minioPortForward, err := e2eutil.PortForwardMinio(kubeCli, restConfig)
		//Expect(err).Should(BeNil())
		//Expect(minioPortForward.Ready(portForwardTimeout)).To(Succeed(), "port-forward should complete within timeout")

		By("create logset cluster with minio provider")
		minioSecret := e2eutil.MinioSecret(env.Namespace)
		Expect(kubeCli.Create(ctx, minioSecret)).To(Succeed())

		minioProvider := e2eutil.MinioShareStorage(minioSecret.Name, "minio-bucket/bucket-test1")
		policyDelete := v1alpha1.PVCRetentionPolicyDelete
		minioProvider.S3.S3RetentionPolicy = &policyDelete
		ls := e2eutil.NewLogSetTpl(env.Namespace, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		ls.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, ls)).To(Succeed())

		var bucket *v1alpha1.BucketClaim
		var err error
		Eventually(func() error {
			bucket, err = v1alpha1.ClaimedBucket(kubeCli, ls)
			if err != nil || bucket == nil {
				return fmt.Errorf("wait bucket creating for logset %v, %v", client.ObjectKeyFromObject(ls), err)
			}
			expectedStatus := v1alpha1.BucketClaimStatus{
				BindTo: v1alpha1.BucketBindToMark(ls),
				State:  v1alpha1.StatusInUse,
			}
			if !reflect.DeepEqual(expectedStatus, bucket.Status) {
				return fmt.Errorf("bucket status is not inuse, current %v", bucket.Status)
			}
			return nil
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		//By("minio bucket path should exist")
		//exist, err := e2eutil.IsMinioPrefixExist("minio-bucket/bucket-test1")
		//Expect(err).Should(BeNil())
		//Expect(exist).Should(BeTrue())

		By("tear down logset cluster")
		Expect(kubeCli.Delete(ctx, ls)).To(Succeed())
		Eventually(func() error {
			return waitLogSetDeleted(ls)
		}, teardownClusterTimeout, pollInterval).Should(Succeed())

		By("bucket should been deleted")
		Eventually(func() error {
			err := kubeCli.Get(ctx, client.ObjectKeyFromObject(bucket), bucket)
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("bucket should be deleted")
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		//By("minio bucket path should not exist")
		//exist, err = e2eutil.IsMinioPrefixExist("minio-bucket/bucket-test1")
		//Expect(err).Should(BeNil())
		//Expect(exist).Should(BeFalse())
		//
		//e2eutil.StopMinioForward(minioPortForward)
	})

	It("Should not delete a bucket which is in use", func() {
		By("create logset cluster with minio provider")
		minioSecret := e2eutil.MinioSecret(env.Namespace)
		Expect(kubeCli.Create(ctx, minioSecret)).To(Succeed())

		minioProvider := e2eutil.MinioShareStorage(minioSecret.Name, "minio-bucket/bucket-test2")
		deletePolicy := v1alpha1.PVCRetentionPolicyDelete
		minioProvider.S3.S3RetentionPolicy = &deletePolicy
		ls := e2eutil.NewLogSetTpl(env.Namespace, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		ls.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, ls)).To(Succeed())

		var bucket *v1alpha1.BucketClaim
		var err error
		Eventually(func() error {
			bucket, err = v1alpha1.ClaimedBucket(kubeCli, ls)
			if err != nil || bucket == nil {
				return fmt.Errorf("wait bucket creating for logset %v, %v", client.ObjectKeyFromObject(ls), err)
			}
			expectedStatus := v1alpha1.BucketClaimStatus{
				BindTo: v1alpha1.BucketBindToMark(ls),
				State:  v1alpha1.StatusInUse,
			}
			if !reflect.DeepEqual(expectedStatus, bucket.Status) {
				return fmt.Errorf("bucket status is not inuse, current %v", bucket.Status)
			}
			return nil
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("bucket should contain inuse condition")
		Expect(kubeCli.Delete(ctx, bucket)).To(Succeed())
		Eventually(func() error {
			bucket, err = v1alpha1.ClaimedBucket(kubeCli, ls)
			if err != nil || bucket == nil {
				return fmt.Errorf("wait bucket creating for logset %v, %v", client.ObjectKeyFromObject(ls), err)
			}
			if len(bucket.Status.Conditions) == 0 {
				return fmt.Errorf("condition should not be empty")
			}
			failCondition := bucket.Status.Conditions[0]
			if failCondition.Type == "recyclable" &&
				failCondition.Status == metav1.ConditionFalse &&
				failCondition.Reason == "InUse" {
				return nil
			}
			return fmt.Errorf("wait inuse condition, current: %v", bucket.Status.Conditions)
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("tear down logset cluster")
		Expect(kubeCli.Delete(ctx, ls)).To(Succeed())
		Eventually(func() error {
			return waitLogSetDeleted(ls)
		}, teardownClusterTimeout, pollInterval).Should(Succeed())

	})
})

func waitLogSetDeleted(ls *v1alpha1.LogSet) error {
	err := kubeCli.Get(ctx, client.ObjectKeyFromObject(ls), ls)
	if err == nil {
		logger.Infow("wait logset teardown", "logset", ls.Name)
		return errWait
	}
	if !apierrors.IsNotFound(err) {
		logger.Errorw("unexpected error when get logset", "logset", ls, "error", err)
		return err
	}
	podList := &corev1.PodList{}
	err = kubeCli.List(ctx, podList, client.InNamespace(ls.Namespace))
	if err != nil {
		logger.Errorw("error list pods", "error", err)
		return err
	}
	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, ls.Name) {
			logger.Infow("Pod that belongs to the logset is not cleaned", "pod", pod.Name)
			return errWait
		}
	}
	return nil
}

func reclaimBucket(bucket *v1alpha1.BucketClaim) error {
	bucket.Finalizers = nil
	if err := kubeCli.Update(ctx, bucket); err != nil {
		return err
	}
	return kubeCli.Delete(ctx, bucket)
}
