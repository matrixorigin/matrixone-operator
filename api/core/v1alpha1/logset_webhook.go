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

package v1alpha1

import (
	"fmt"
	"time"

	"github.com/matrixorigin/matrixone-operator/api/features"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	defaultShardNum = 1

	minHAReplicas = 3
	singleReplica = 1

	defaultStoreFailureTimeout = 10 * time.Minute
)

var (
	kClient client.Client
)

func (r *LogSet) setupWebhookWithManager(mgr ctrl.Manager) error {
	kClient = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-logset,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=logsets,verbs=create;update,versions=v1alpha1,name=mlogset.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &LogSet{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *LogSet) Default() {
	r.Spec.Default()
}

func (r *LogSetSpec) Default() {
	//if r.InitialConfig.HAKeeperReplicas == nil {
	//	if r.Replicas >= minHAReplicas {
	//		r.InitialConfig.HAKeeperReplicas = pointer.Int(minHAReplicas)
	//	} else {
	//		r.InitialConfig.HAKeeperReplicas = pointer.Int(singleReplica)
	//	}
	//}
	if r.InitialConfig.LogShardReplicas == nil {
		if r.Replicas >= minHAReplicas {
			r.InitialConfig.LogShardReplicas = pointer.Int(minHAReplicas)
		} else {
			r.InitialConfig.LogShardReplicas = pointer.Int(singleReplica)
		}
	}
	if r.InitialConfig.LogShards == nil {
		r.InitialConfig.LogShards = pointer.Int(defaultShardNum)
	}
	if r.InitialConfig.DNShards == nil {
		r.InitialConfig.DNShards = pointer.Int(defaultShardNum)
	}
	if r.StoreFailureTimeout == nil {
		r.StoreFailureTimeout = &metav1.Duration{Duration: defaultStoreFailureTimeout}
	}
	r.setDefaultRetentionPolicy()
	setDefaultServiceArgs(r)
	setPodSetDefaults(&r.PodSet)
}

// setDefaultRetentionPolicy always set PVCRetentionPolicy, and always set S3RetentionPolicy only if S3 is not nil
// setDefaultRetentionPolicy does not change origin policy and only set default value when policy is nil
func (r *LogSetSpec) setDefaultRetentionPolicy() {
	defaultDeletePolicy := PVCRetentionPolicyDelete

	if r.SharedStorage.S3 == nil {
		if r.PVCRetentionPolicy == nil {
			r.PVCRetentionPolicy = &defaultDeletePolicy
		}
		return
	}

	pvcPolicy := r.PVCRetentionPolicy
	s3Policy := r.SharedStorage.S3.S3RetentionPolicy

	switch {
	// if both set, does not set any values
	case pvcPolicy != nil && s3Policy != nil:
		return
	// if both not set, set to delete
	case pvcPolicy == nil && s3Policy == nil:
		r.PVCRetentionPolicy = &defaultDeletePolicy
		r.SharedStorage.S3.S3RetentionPolicy = &defaultDeletePolicy
	// if only set pvcPolicy, set it to s3Policy
	case pvcPolicy != nil && s3Policy == nil:
		r.SharedStorage.S3.S3RetentionPolicy = pvcPolicy
	// if only set s3Policy, set it to pvcPolicy
	case pvcPolicy == nil && s3Policy != nil:
		r.PVCRetentionPolicy = s3Policy
	}
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-logset,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=logsets,verbs=create;update,versions=v1alpha1,name=vlogset.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &LogSet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *LogSet) ValidateCreate() (admission.Warnings, error) {
	errs := r.Spec.ValidateCreate(r.ObjectMeta)
	errs = append(errs, validateMainContainer(&r.Spec.MainContainer, field.NewPath("spec"))...)
	return nil, invalidOrNil(errs, r)
}

func (r *LogSet) ValidateUpdate(o runtime.Object) (admission.Warnings, error) {
	old := o.(*LogSet)
	errs := r.Spec.ValidateUpdate(&old.Spec, r.ObjectMeta)
	return nil, invalidOrNil(errs, r)
}

func (r *LogSet) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *LogSetSpec) validateMutateCommon() field.ErrorList {
	var errs field.ErrorList
	errs = append(errs, validateVolume(&r.Volume, field.NewPath("spec").Child("volume"))...)
	errs = append(errs, r.validateInitialConfig()...)
	errs = append(errs, r.validateSharedStorage()...)
	errs = append(errs, validateGoMemLimitPercent(r.MemoryLimitPercent, field.NewPath("spec").Child("memoryLimitPercent"))...)
	return errs
}

func (r *LogSetSpec) ValidateCreate(meta metav1.ObjectMeta) field.ErrorList {
	var errs field.ErrorList
	errs = append(errs, r.validateMutateCommon()...)
	errs = append(errs, r.validateIfBucketInUse(meta)...)
	errs = append(errs, r.validateIfBucketDeleting()...)
	return errs
}

func (r *LogSetSpec) ValidateUpdate(old *LogSetSpec, meta metav1.ObjectMeta) field.ErrorList {
	if err := r.validateMutateCommon(); err != nil {
		return err
	}
	var errs field.ErrorList
	if !equality.Semantic.DeepEqual(old.InitialConfig, r.InitialConfig) {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("initialConfig"), nil, "initialConfig is immutable"))
	}
	errs = append(errs, r.validateIfBucketInUse(meta)...)
	return errs
}

func (r *LogSetSpec) validateSharedStorage() field.ErrorList {
	var errs field.ErrorList
	parent := field.NewPath("spec").Child("sharedStorage")
	count := 0
	if r.SharedStorage.S3 != nil {
		count += 1
		if r.SharedStorage.S3.Path == "" {
			errs = append(errs, field.Invalid(parent, nil, "path must be set for S3 storage"))
		}
	}
	if r.SharedStorage.FileSystem != nil {
		count += 1
		if r.SharedStorage.FileSystem.Path == "" {
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

func (r *LogSetSpec) validateInitialConfig() field.ErrorList {
	var errs field.ErrorList
	parent := field.NewPath("spec").Child("initialConfig")

	//if hrs := r.InitialConfig.HAKeeperReplicas; hrs == nil {
	//	errs = append(errs, field.Invalid(parent.Child("haKeeperReplicas"), hrs, "haKeeperReplicas must be set"))
	//} else if *hrs > int(r.Replicas) {
	//	errs = append(errs, field.Invalid(parent.Child("haKeeperReplicas"), hrs, "haKeeperReplicas must not larger then logservice replicas"))
	//}

	if lrs := r.InitialConfig.LogShardReplicas; lrs == nil {
		errs = append(errs, field.Invalid(parent.Child("logShardReplicas"), lrs, "logShardReplicas must be set"))
	} else if *lrs > int(r.Replicas) {
		errs = append(errs, field.Invalid(parent.Child("logShardReplicas"), lrs, "logShardReplicas must not larger then logservice replicas"))
	}

	if lss := r.InitialConfig.LogShards; lss == nil {
		errs = append(errs, field.Invalid(parent.Child("logShards"), lss, "logShards must be set"))
	}

	if dss := r.InitialConfig.DNShards; dss == nil {
		errs = append(errs, field.Invalid(parent.Child("dnShards"), dss, "dnShards must be set"))
	}
	return errs
}

func (r *LogSetSpec) validateIfBucketDeleting() field.ErrorList {
	if !features.DefaultFeatureGate.Enabled(features.S3Reclaim) {
		return nil
	}
	if r.SharedStorage.S3 == nil {
		return nil
	}
	var errs field.ErrorList
	path := field.NewPath("spec").Child("sharedStorage").Child("s3")
	bucket, err := ClaimedBucket(kClient, r.SharedStorage.S3)
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

func (r *LogSetSpec) validateIfBucketInUse(meta metav1.ObjectMeta) field.ErrorList {
	if !features.DefaultFeatureGate.Enabled(features.S3Reclaim) {
		return nil
	}
	if r.SharedStorage.S3 == nil {
		return nil
	}
	var errs field.ErrorList
	path := field.NewPath("spec").Child("sharedStorage").Child("s3")
	bucket, err := ClaimedBucket(kClient, r.SharedStorage.S3)
	if err != nil {
		errs = append(errs, field.Invalid(path, nil, err.Error()))
		return errs
	}
	if bucket == nil {
		return nil
	}
	if bucket.Status.State == StatusInUse &&
		bucket.Status.BindTo != BucketBindToMark(meta) {
		msg := fmt.Sprintf("claimed bucket %v already bind to %v", client.ObjectKeyFromObject(bucket), bucket.Status.BindTo)
		errs = append(errs, field.Invalid(path, nil, msg))
		return errs
	}
	return nil
}
