// Copyright 2023 Matrix Origin
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
	"fmt"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sync"
)

var _ recon.Actor[*v1alpha1.BucketClaim] = &Actor{}

type Actor struct {
	mutex  sync.Mutex
	tasks  chan task
	client client.Client
}

type task struct {
	bucketClaim client.ObjectKey
	job         *batchv1.Job
}

func (bca *Actor) Observe(ctx *recon.Context[*v1alpha1.BucketClaim]) (recon.Action[*v1alpha1.BucketClaim], error) {
	ctx.Log.Info(fmt.Sprintf("observe bucketclaim %v", client.ObjectKeyFromObject(ctx.Obj)))
	return nil, nil
}

func (bca *Actor) Finalize(ctx *recon.Context[*v1alpha1.BucketClaim]) (bool, error) {
	ctx.Log.Info(fmt.Sprintf("finalize bucketclaim %v", client.ObjectKeyFromObject(ctx.Obj)))

	bucket := ctx.Obj
	if bucket.Status.State == v1alpha1.StatusInUse {
		failCondition := newFailCondition("InUse", "bucket is in inuse, cannot be deleted")
		bucket.Status.ConditionalStatus.Conditions = []metav1.Condition{*failCondition}
		return false, ctx.Update(bucket)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName(bucket),
			Namespace: bucket.Namespace,
		},
	}
	err := ctx.Get(client.ObjectKeyFromObject(job), job)
	switch {
	case apierrors.IsNotFound(err):
		return false, bca.createNewJob(ctx)
	case err != nil:
		return false, err
	}

	if isJobSuccess(job) {
		// successful delete data from s3, remove finalizer, remove bucketclaim from API server
		bucket.Finalizers = removeFromSlice(bucket.Finalizers, v1alpha1.BucketDataFinalizer)
		if err = ctx.Update(bucket); err != nil {
			return false, err
		}
		return true, nil
	}
	if isJobFailure(job) {
		failCondition := newFailCondition("JobFailure", fmt.Sprintf("s3 job failure: %v", client.ObjectKeyFromObject(job)))
		bucket.Status.ConditionalStatus.Conditions = []metav1.Condition{*failCondition}
		return false, ctx.Update(bucket)
	}
	// wait job finished backend
	return false, nil
}

func (bca *Actor) createNewJob(ctx *recon.Context[*v1alpha1.BucketClaim]) error {
	bucket := ctx.Obj

	// create configmap before creating job, which includes the entrypoint of job container
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName(bucket),
			Namespace: bucket.Namespace,
		},
	}
	err := ctx.Get(client.ObjectKeyFromObject(cm), cm)
	switch {
	case apierrors.IsNotFound(err):
		// create configmap if not exist, then try to create job
		cm, err = bca.NewCmTpl(bucket)
		if err != nil {
			return err
		}
		if err = ctx.CreateOwned(cm); err != nil {
			return err
		}
	case err != nil:
		return err
	}

	job := bca.NewJobTpl(bucket, cm)
	if err = ctx.CreateOwned(job); err != nil {
		failCondition := newFailCondition("FailCreateJob", err.Error())
		bucket.Status.ConditionalStatus.Conditions = []metav1.Condition{*failCondition}
		return ctx.Update(bucket)
	}
	return nil
}

func isJobSuccess(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	if job.Status.Succeeded >= 1 {
		return true
	}
	return false
}

func isJobFailure(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func removeFromSlice(ss []string, s string) []string {
	var ns []string
	for _, v := range ss {
		if v != s {
			ns = append(ns, v)
		}
	}
	return ns
}

func newFailCondition(reason, message string) *metav1.Condition {
	failCondition := &metav1.Condition{
		Type:               "recyclable",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
	return failCondition
}

func (bca *Actor) Reconcile(mgr manager.Manager) error {
	return recon.Setup[*v1alpha1.BucketClaim](&v1alpha1.BucketClaim{}, "BucketClaim", mgr, bca,
		recon.WithBuildFn(func(b *builder.Builder) {
			b.Owns(&batchv1.Job{})
		}))
}
