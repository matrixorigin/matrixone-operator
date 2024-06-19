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

package webhook

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/api/features"
)

const (
	defaultShardNum = 1

	minHAReplicas = 3
	singleReplica = 1

	defaultStoreFailureTimeout = 10 * time.Minute
)

type logSetWebhook struct{}

func (logSetWebhook) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.LogSet{}).
		WithDefaulter(&logSetDefaulter{}).
		WithValidator(&logSetValidator{
			kClient: mgr.GetClient(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-logset,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=logsets,verbs=create;update,versions=v1alpha1,name=mlogset.kb.io,admissionReviewVersions={v1,v1beta1}

// logSetDefaulter implements webhook.Defaulter so a webhook will be registered for v1alpha1.LogSet
type logSetDefaulter struct{}

var _ webhook.CustomDefaulter = &logSetDefaulter{}

func (l *logSetDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	logSet, ok := obj.(*v1alpha1.LogSet)
	if !ok {
		return unexpectedKindError("LogSet", obj)
	}
	l.DefaultSpec(&logSet.Spec)
	return nil
}

func (l *logSetDefaulter) DefaultSpec(spec *v1alpha1.LogSetSpec) {
	//if r.InitialConfig.HAKeeperReplicas == nil {
	//	if r.Replicas >= minHAReplicas {
	//		r.InitialConfig.HAKeeperReplicas = pointer.Int(minHAReplicas)
	//	} else {
	//		r.InitialConfig.HAKeeperReplicas = pointer.Int(singleReplica)
	//	}
	//}
	if spec.InitialConfig.LogShardReplicas == nil {
		if spec.Replicas >= minHAReplicas {
			spec.InitialConfig.LogShardReplicas = pointer.Int(minHAReplicas)
		} else {
			spec.InitialConfig.LogShardReplicas = pointer.Int(singleReplica)
		}
	}
	if spec.InitialConfig.LogShards == nil {
		spec.InitialConfig.LogShards = pointer.Int(defaultShardNum)
	}
	if spec.InitialConfig.DNShards == nil {
		spec.InitialConfig.DNShards = pointer.Int(defaultShardNum)
	}
	if spec.StoreFailureTimeout == nil {
		spec.StoreFailureTimeout = &metav1.Duration{Duration: defaultStoreFailureTimeout}
	}
	l.setDefaultRetentionPolicy(spec)
	setDefaultServiceArgs(spec)
	setPodSetDefaults(&spec.PodSet)
}

// setDefaultRetentionPolicy always set PVCRetentionPolicy, and always set S3RetentionPolicy only if S3 is not nil
// setDefaultRetentionPolicy does not change origin policy and only set default value when policy is nil
func (l *logSetDefaulter) setDefaultRetentionPolicy(spec *v1alpha1.LogSetSpec) {
	defaultDeletePolicy := v1alpha1.PVCRetentionPolicyDelete

	if spec.SharedStorage.S3 == nil {
		if spec.PVCRetentionPolicy == nil {
			spec.PVCRetentionPolicy = &defaultDeletePolicy
		}
		return
	}

	pvcPolicy := spec.PVCRetentionPolicy
	s3Policy := spec.SharedStorage.S3.S3RetentionPolicy

	switch {
	// if both set, does not set any values
	case pvcPolicy != nil && s3Policy != nil:
		return
	// if both not set, set to delete
	case pvcPolicy == nil && s3Policy == nil:
		spec.PVCRetentionPolicy = &defaultDeletePolicy
		spec.SharedStorage.S3.S3RetentionPolicy = &defaultDeletePolicy
	// if only set pvcPolicy, set it to s3Policy
	case pvcPolicy != nil && s3Policy == nil:
		spec.SharedStorage.S3.S3RetentionPolicy = pvcPolicy
	// if only set s3Policy, set it to pvcPolicy
	case pvcPolicy == nil && s3Policy != nil:
		spec.PVCRetentionPolicy = s3Policy
	}
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-logset,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=logsets,verbs=create;update,versions=v1alpha1,name=vlogset.kb.io,admissionReviewVersions={v1,v1beta1}

// logSetValidator implements webhook.Validator so a webhook will be registered for the v1alpha1.LogSet
type logSetValidator struct {
	kClient client.Client
}

var _ webhook.CustomValidator = &logSetValidator{}

func (l *logSetValidator) ValidateCreate(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	logSet, ok := obj.(*v1alpha1.LogSet)
	if !ok {
		return nil, unexpectedKindError("LogSet", obj)
	}
	errs := l.ValidateSpecCreate(logSet.ObjectMeta, &logSet.Spec)
	errs = append(errs, validateMainContainer(&logSet.Spec.MainContainer, field.NewPath("spec"))...)
	return nil, invalidOrNil(errs, logSet)
}

func (l *logSetValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	old := oldObj.(*v1alpha1.LogSet)
	logSet := newObj.(*v1alpha1.LogSet)
	errs := l.ValidateSpecUpdate(&old.Spec, &logSet.Spec, logSet.ObjectMeta)
	return nil, invalidOrNil(errs, logSet)
}

func (l *logSetValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (l *logSetValidator) ValidateSpecCreate(meta metav1.ObjectMeta, spec *v1alpha1.LogSetSpec) field.ErrorList {
	var errs field.ErrorList
	errs = append(errs, l.validateMutateCommon(spec)...)
	errs = append(errs, l.validateIfBucketInUse(meta, spec)...)
	errs = append(errs, l.validateIfBucketDeleting(spec)...)
	return errs
}

func (l *logSetValidator) ValidateSpecUpdate(oldSpec, spec *v1alpha1.LogSetSpec, meta metav1.ObjectMeta) field.ErrorList {
	if err := l.validateMutateCommon(spec); err != nil {
		return err
	}
	var errs field.ErrorList
	if !equality.Semantic.DeepEqual(oldSpec.InitialConfig, spec.InitialConfig) {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("initialConfig"), nil, "initialConfig is immutable"))
	}
	errs = append(errs, l.validateIfBucketInUse(meta, spec)...)
	return errs
}

func (l *logSetValidator) validateMutateCommon(spec *v1alpha1.LogSetSpec) field.ErrorList {
	var errs field.ErrorList
	errs = append(errs, validateVolume(&spec.Volume, field.NewPath("spec").Child("volume"))...)
	errs = append(errs, l.validateInitialConfig(spec)...)
	errs = append(errs, l.validateSharedStorage(spec)...)
	errs = append(errs, validateGoMemLimitPercent(spec.MemoryLimitPercent, field.NewPath("spec").Child("memoryLimitPercent"))...)
	return errs
}

func (l *logSetValidator) validateSharedStorage(spec *v1alpha1.LogSetSpec) field.ErrorList {
	var errs field.ErrorList
	parent := field.NewPath("spec").Child("sharedStorage")
	count := 0
	if spec.SharedStorage.S3 != nil {
		count += 1
		if spec.SharedStorage.S3.Path == "" {
			errs = append(errs, field.Invalid(parent, nil, "path must be set for S3 storage"))
		}
	}
	if spec.SharedStorage.FileSystem != nil {
		count += 1
		if spec.SharedStorage.FileSystem.Path == "" {
			errs = append(errs, field.Invalid(parent, nil, "path must be set for file-system storage"))
		}
	}
	if count < 1 {
		errs = append(errs, field.Invalid(parent, nil, "no shared storage provider configured"))
	}
	if count > 1 {
		errs = append(errs, field.Invalid(parent, nil, "more than 1 storage provider configured"))
	}
	return errs
}

func (l *logSetValidator) validateInitialConfig(spec *v1alpha1.LogSetSpec) field.ErrorList {
	var errs field.ErrorList
	parent := field.NewPath("spec").Child("initialConfig")

	//if hrs := r.InitialConfig.HAKeeperReplicas; hrs == nil {
	//	errs = append(errs, field.Invalid(parent.Child("haKeeperReplicas"), hrs, "haKeeperReplicas must be set"))
	//} else if *hrs > int(r.Replicas) {
	//	errs = append(errs, field.Invalid(parent.Child("haKeeperReplicas"), hrs, "haKeeperReplicas must not larger then logservice replicas"))
	//}

	if lrs := spec.InitialConfig.LogShardReplicas; lrs == nil {
		errs = append(errs, field.Invalid(parent.Child("logShardReplicas"), lrs, "logShardReplicas must be set"))
	} else if *lrs > int(spec.Replicas) {
		errs = append(errs, field.Invalid(parent.Child("logShardReplicas"), lrs, "logShardReplicas must not larger then logservice replicas"))
	}

	if lss := spec.InitialConfig.LogShards; lss == nil {
		errs = append(errs, field.Invalid(parent.Child("logShards"), lss, "logShards must be set"))
	}

	if dss := spec.InitialConfig.DNShards; dss == nil {
		errs = append(errs, field.Invalid(parent.Child("dnShards"), dss, "dnShards must be set"))
	}
	return errs
}

func (l *logSetValidator) validateIfBucketDeleting(spec *v1alpha1.LogSetSpec) field.ErrorList {
	if !features.DefaultFeatureGate.Enabled(features.S3Reclaim) {
		return nil
	}
	if spec.SharedStorage.S3 == nil {
		return nil
	}
	var errs field.ErrorList
	path := field.NewPath("spec").Child("sharedStorage").Child("s3")
	bucket, err := v1alpha1.ClaimedBucket(l.kClient, spec.SharedStorage.S3)
	if err != nil {
		errs = append(errs, field.Invalid(path, nil, err.Error()))
		return errs
	}
	if bucket == nil {
		return nil
	}
	if bucket.DeletionTimestamp != nil {
		msg := fmt.Sprintf("claimed bucket %v state is deleting", client.ObjectKeyFromObject(bucket))
		errs = append(errs, field.Invalid(path, nil, msg))
		return errs
	}
	return nil
}

func (l *logSetValidator) validateIfBucketInUse(meta metav1.ObjectMeta, spec *v1alpha1.LogSetSpec) field.ErrorList {
	if !features.DefaultFeatureGate.Enabled(features.S3Reclaim) {
		return nil
	}
	if spec.SharedStorage.S3 == nil {
		return nil
	}
	var errs field.ErrorList
	path := field.NewPath("spec").Child("sharedStorage").Child("s3")
	bucket, err := v1alpha1.ClaimedBucket(l.kClient, spec.SharedStorage.S3)
	if err != nil {
		errs = append(errs, field.Invalid(path, nil, err.Error()))
		return errs
	}
	if bucket == nil {
		return nil
	}
	if bucket.Status.State == v1alpha1.StatusInUse &&
		bucket.Status.BindTo != v1alpha1.BucketBindToMark(meta) {
		msg := fmt.Sprintf("claimed bucket %v already bind to %v", client.ObjectKeyFromObject(bucket), bucket.Status.BindTo)
		errs = append(errs, field.Invalid(path, nil, msg))
		return errs
	}
	return nil
}
