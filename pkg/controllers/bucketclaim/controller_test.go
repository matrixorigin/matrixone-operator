// Copyright 2025-2026 Matrix Origin
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

package bucketclaim

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/matrixorigin/controller-runtime/pkg/fake"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"
)

func TestActor_Finalize_ReleasedSkipsS3CleanupJob(t *testing.T) {
	g := NewGomegaWithT(t)

	now := metav1.NewTime(time.Now())
	bucket := &v1alpha1.BucketClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-bucket",
			Namespace:         "default",
			Finalizers:        []string{v1alpha1.BucketDataFinalizer},
			DeletionTimestamp: &now,
			Annotations: map[string]string{
				v1alpha1.AnnAnyInstanceRunning: "true",
			},
		},
		Spec: v1alpha1.BucketClaimSpec{
			S3: &v1alpha1.S3Provider{Path: "minio-bucket/test"},
		},
		Status: v1alpha1.BucketClaimStatus{
			State: v1alpha1.StatusReleased,
		},
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(batchv1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))

	cli := fake.KubeClientBuilder().WithScheme(scheme).WithObjects(bucket).Build()

	mockCtrl := gomock.NewController(t)
	eventEmitter := fake.NewMockEventEmitter(mockCtrl)
	ctx := fake.NewContext(bucket, cli, eventEmitter)

	actor := New()
	ok, err := actor.Finalize(ctx)
	g.Expect(err).To(Succeed())
	g.Expect(ok).To(BeTrue())

	updated := &v1alpha1.BucketClaim{}
	err = cli.Get(context.TODO(), client.ObjectKeyFromObject(bucket), updated)
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	jobs := &batchv1.JobList{}
	g.Expect(cli.List(context.TODO(), jobs, client.InNamespace(bucket.Namespace))).To(Succeed())
	g.Expect(jobs.Items).To(BeEmpty())
}

func TestParseEndpoint(t *testing.T) {
	s3 := v1alpha1.S3Provider{
		Path:     "minio-mo/test",
		Endpoint: "http://minio.mostorage:9000",
		Region:   "us-east-1",
	}
	expect := `
#!/bin/sh

set -ex

region='us-east-1'
endpoint='http://minio.mostorage:9000'

if [ -n "${region}" ]; then
  export AWS_REGION="${region}"
fi

if [ -n "${endpoint}" ]; then
  aws --endpoint-url "${endpoint}" s3api head-bucket --bucket 'minio-mo'
  aws --endpoint-url "${endpoint}" s3 rm 's3://minio-mo/test' --recursive
else
  aws s3api head-bucket --bucket 'minio-mo'
  aws s3 rm 's3://minio-mo/test' --recursive
fi
`
	endpoint, err := parseEntrypoint(&s3)
	g := NewGomegaWithT(t)
	g.Expect(err).To(Succeed())
	g.Expect(endpoint).To(Equal(expect))
}

func TestFinalizeWaitsForVerifiedCleanup(t *testing.T) {
	for _, tt := range []struct {
		name      string
		condition batchv1.JobConditionType
		wantDone  bool
	}{
		{"running", "", false},
		{"failed cleanup retains finalizer", batchv1.JobFailed, false},
		{"successful cleanup releases finalizer", batchv1.JobComplete, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			now := metav1.NewTime(time.Now())
			bucket := &v1alpha1.BucketClaim{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test", Finalizers: []string{v1alpha1.BucketDataFinalizer}, DeletionTimestamp: &now, Annotations: map[string]string{v1alpha1.AnnAnyInstanceRunning: "true"}},
				Status:     v1alpha1.BucketClaimStatus{State: v1alpha1.StatusDeleting},
			}
			job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: jobName(bucket), Namespace: bucket.Namespace}}
			if tt.condition != "" {
				job.Status.Conditions = []batchv1.JobCondition{{Type: tt.condition, Status: corev1.ConditionTrue}}
			}
			scheme := runtime.NewScheme()
			g.Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())
			g.Expect(batchv1.AddToScheme(scheme)).To(Succeed())
			g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
			cli := fake.KubeClientBuilder().WithScheme(scheme).WithObjects(bucket, job).Build()
			ctx := fake.NewContext(bucket, cli, fake.NewMockEventEmitter(gomock.NewController(t)))
			done, err := New().Finalize(ctx)
			g.Expect(err).To(Succeed())
			g.Expect(done).To(Equal(tt.wantDone))
			updated := &v1alpha1.BucketClaim{}
			err = cli.Get(context.Background(), client.ObjectKeyFromObject(bucket), updated)
			if tt.wantDone {
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
				return
			}
			g.Expect(err).To(Succeed())
			g.Expect(updated.Finalizers).To(ContainElement(v1alpha1.BucketDataFinalizer))
			if tt.condition == batchv1.JobFailed {
				g.Expect(updated.Status.Conditions[0].Reason).To(Equal("JobFailure"))
			}
		})
	}
}
