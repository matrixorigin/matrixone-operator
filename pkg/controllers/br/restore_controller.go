// Copyright 2024 Matrix Origin
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

package br

import (
	"fmt"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/cmd"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"strings"
)

type RestoreActor struct {
	restoreImage string
}

func NewRestoreActor(image string) *RestoreActor {
	return &RestoreActor{restoreImage: image}
}

var _ recon.Actor[*v1alpha1.RestoreJob] = &RestoreActor{}

func (c *RestoreActor) Observe(ctx *recon.Context[*v1alpha1.RestoreJob]) (recon.Action[*v1alpha1.RestoreJob], error) {
	phase := ctx.Obj.GetPhase()
	if phase == v1alpha1.JobPhaseFailed || phase == v1alpha1.JobPhaseCompleted {
		// completed
		return nil, nil
	}
	if phase == v1alpha1.JobPhaseRunning {
		return c.waitJob, nil
	}
	return c.syncJob, nil
}

func (c *RestoreActor) waitJob(ctx *recon.Context[*v1alpha1.RestoreJob]) error {
	rj := ctx.Obj
	job := &batchv1.Job{}
	if err := ctx.Get(types.NamespacedName{Namespace: rj.Namespace, Name: rj.Name}, job); err != nil {
		if apierrors.IsNotFound(err) {
			return c.failRestore(ctx, "job is cleaned externally")
		}
		return errors.Wrap(err, "error get backup job")
	}
	if job.Status.Failed > 0 {
		return c.failRestore(ctx, "backup job is failed")
	}
	svc := buildSvc(rj)
	status, err := cmd.GetCmdStatus(fmt.Sprintf("%s.%s", svc.Name, svc.Namespace), defaultCMDRestPort)
	if err != nil {
		return errors.Wrap(err, "error get restore status")
	}
	if !status.Completed {
		return recon.ErrReSync("wait restore complete", pollInterval)
	}
	// restore succeed
	if status.ExitCode == 0 {
		return c.successRestore(ctx)
	}
	return c.failRestore(ctx, status.Stderr)
}

func (c *RestoreActor) failRestore(ctx *recon.Context[*v1alpha1.RestoreJob], msg string) error {
	ctx.Obj.Status.Phase = v1alpha1.JobPhaseFailed
	meta.SetStatusCondition(&ctx.Obj.Status.Conditions, metav1.Condition{
		Type:    v1alpha1.JobConditionTypeEnded,
		Status:  metav1.ConditionTrue,
		Reason:  "JobFailed",
		Message: msg,
	})
	return ctx.UpdateStatus(ctx.Obj)
}

func (c *RestoreActor) successRestore(ctx *recon.Context[*v1alpha1.RestoreJob]) error {
	rj := ctx.Obj
	if err := util.Ignore(apierrors.IsNotFound, ctx.Delete(
		&batchv1.Job{ObjectMeta: common.ObjMetaTemplate(rj, rj.Name)},
		client.PropagationPolicy(metav1.DeletePropagationBackground),
	)); err != nil {
		return errors.Wrap(err, "error finalize restore job")
	}
	rj.Status.Phase = v1alpha1.JobPhaseCompleted
	meta.SetStatusCondition(&rj.Status.Conditions, metav1.Condition{
		Type:   v1alpha1.JobConditionTypeEnded,
		Status: metav1.ConditionTrue,
		Reason: "JobComplete",
	})
	return ctx.UpdateStatus(rj)
}

func (c *RestoreActor) syncJob(ctx *recon.Context[*v1alpha1.RestoreJob]) error {
	rj := ctx.Obj
	if rj.Status.Phase == "" {
		rj.Status.Phase = v1alpha1.JobPhasePending
	}
	restoreCmd := &RestoreCommand{}
	backup := &v1alpha1.Backup{}
	if err := ctx.Get(types.NamespacedName{Name: rj.Spec.BackupName}, backup); err != nil {
		return errors.Wrap(err, "error get backup")
	}
	restoreCmd.BackupID = backup.Meta.ID
	optionalSourceSecret := backup.Meta.Location.S3.SecretRef
	restoreCmd.ReadSourceEnvSecret = optionalSourceSecret != nil
	restoreCmd.Target.Endpoint = rj.Spec.Target.S3.Endpoint
	restoreCmd.Target.Type = string(rj.Spec.Target.S3.GetProviderType())
	parts := strings.SplitN(rj.Spec.Target.S3.Path, "/", 2)
	restoreCmd.Target.Bucket = parts[0]
	if len(parts) > 1 {
		restoreCmd.Target.Path = parts[1]
	}
	optionalTargetSecret := rj.Spec.Target.S3.SecretRef
	restoreCmd.Target.ReadEnvSecret = optionalTargetSecret != nil
	job := buildJob(rj, c.restoreImage, restoreCmd.String(), func(c *corev1.Container) {
		c.Env = []corev1.EnvVar{{
			Name:  RawMetaEnv,
			Value: backup.Meta.Raw,
		}}
		if optionalSourceSecret != nil {
			for _, key := range []string{common.AWSAccessKeyID, common.AWSSecretAccessKey} {
				c.Env = util.UpsertByKey(c.Env, corev1.EnvVar{Name: key, ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: *optionalSourceSecret,
						Key:                  key,
					},
				}}, util.EnvVarKey)
			}
		}
		if optionalTargetSecret != nil {
			c.Env = util.UpsertByKey(c.Env, corev1.EnvVar{Name: RestoreAccessEnvKey, ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: *optionalSourceSecret,
					Key:                  common.AWSAccessKeyID,
				},
			}}, util.EnvVarKey)
			c.Env = util.UpsertByKey(c.Env, corev1.EnvVar{Name: RestoreSecretEnvKey, ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: *optionalSourceSecret,
					Key:                  common.AWSSecretAccessKey,
				},
			}}, util.EnvVarKey)
		}
	})
	svc := buildSvc(rj)
	if err := util.Ignore(apierrors.IsAlreadyExists, ctx.CreateOwned(job)); err != nil {
		return errors.Wrap(err, "error ensure job")
	}
	if err := util.Ignore(apierrors.IsAlreadyExists, ctx.CreateOwned(svc)); err != nil {
		return errors.Wrap(err, "error ensure service")
	}
	rj.Status.Phase = v1alpha1.JobPhaseRunning
	return ctx.UpdateStatus(rj)
}

func (c *RestoreActor) Finalize(ctx *recon.Context[*v1alpha1.RestoreJob]) (bool, error) {
	rj := ctx.Obj
	err := ctx.Delete(&batchv1.Job{ObjectMeta: common.ObjMetaTemplate(rj, rj.Name)}, client.PropagationPolicy(metav1.DeletePropagationBackground))
	if err == nil {
		// check next time
		return false, nil
	}
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	return false, errors.Wrap(err, "error delete job")
}

func (c *RestoreActor) Reconcile(mgr manager.Manager) error {
	err := recon.Setup[*v1alpha1.RestoreJob](&v1alpha1.RestoreJob{}, "restorejob", mgr, c, recon.WithBuildFn(func(b *builder.Builder) {
		b.Owns(&batchv1.Job{})
	}))
	if err != nil {
		return err
	}
	return nil
}
