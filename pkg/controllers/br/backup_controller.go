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
	manager "sigs.k8s.io/controller-runtime/pkg/manager"
	"strings"
	"time"
)

const (
	pollInterval = 15 * time.Second
)

type BackupActor struct {
	backupImage string
}

func NewBackupActor(image string) *BackupActor {
	return &BackupActor{backupImage: image}
}

var _ recon.Actor[*v1alpha1.BackupJob] = &BackupActor{}

func (c *BackupActor) Observe(ctx *recon.Context[*v1alpha1.BackupJob]) (recon.Action[*v1alpha1.BackupJob], error) {
	bj := ctx.Obj
	cond, ok := recon.GetCondition(bj, v1alpha1.JobConditionTypeEnded)
	if ok && cond.Status == metav1.ConditionTrue {
		// completed
		return nil, nil
	}
	if bj.Status.Phase == v1alpha1.JobPhaseRunning {
		return c.waitJob, nil
	}
	return c.syncJob, nil
}

func (c *BackupActor) waitJob(ctx *recon.Context[*v1alpha1.BackupJob]) error {
	bj := ctx.Obj
	// check whether the backup has completed
	preLabels := map[string]string{
		common.PreNameLabelKey: bj.Name,
		common.PreUUIDLabelKey: string(bj.UID),
	}
	backupList := &v1alpha1.BackupList{}
	if err := ctx.List(backupList, client.MatchingLabels(preLabels)); err != nil {
		return errors.Wrap(err, "error list owned backups")
	}
	if len(backupList.Items) > 0 {
		return c.completeBackup(ctx, &backupList.Items[0])
	}

	job := &batchv1.Job{}
	if err := ctx.Get(types.NamespacedName{Namespace: bj.Namespace, Name: bj.Name}, job); err != nil {
		if apierrors.IsNotFound(err) {
			return c.failBackup(ctx, "job is cleaned externally")
		}
		return errors.Wrap(err, "error get backup job")
	}
	if job.Status.Failed > 0 {
		return c.failBackup(ctx, "backup job is failed")
	}

	svc := buildSvc(bj)
	status, err := cmd.GetCmdStatus(fmt.Sprintf("%s.%s", svc.Name, svc.Namespace), defaultCMDRestPort)
	if err != nil {
		return errors.Wrap(err, "error get backup status")
	}
	if !status.Completed {
		meta.SetStatusCondition(&bj.Status.Conditions, metav1.Condition{
			Type:   v1alpha1.JobConditionTypeEnded,
			Status: metav1.ConditionFalse,
			Reason: "JobRunning",
		})
		return recon.ErrReSync("wait backup complete", pollInterval)
	}
	// backup succeed
	if status.ExitCode == 0 {
		parts := strings.Split(status.Stdout, MetaDelimiter)
		if len(parts) != 2 {
			return errors.Errorf("error parse backup id from stdout: %s", status.Stdout)
		}
		raw := strings.Trim(strings.Trim(parts[1], " "), "\n")
		id := strings.SplitN(raw, ",", 2)[0]

		// 1. ensure backup
		backup := &v1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%s", bj.Name, id[:5]),
			},
			Meta: v1alpha1.BackupMeta{
				Location: bj.Spec.Target,
				ID:       id,
				// TODO: get AtTime from backup Meta
				AtTime:       metav1.Now(),
				CompleteTime: metav1.Now(),
				SourceRef:    bj.GetSourceRef(),
				Raw:          raw,
			},
		}
		if err := util.Ignore(apierrors.IsAlreadyExists, ctx.Create(backup)); err != nil {
			return errors.Wrap(err, "error ensure backup")
		}
		return c.completeBackup(ctx, backup)
	}

	return c.failBackup(ctx, status.Stderr)
}

func (c *BackupActor) failBackup(ctx *recon.Context[*v1alpha1.BackupJob], message string) error {
	// note: when backup failed, we keep the job for troubleshooting
	ctx.Obj.Status.Phase = v1alpha1.JobPhaseFailed
	meta.SetStatusCondition(&ctx.Obj.Status.Conditions, metav1.Condition{
		Type:    v1alpha1.JobConditionTypeEnded,
		Status:  metav1.ConditionTrue,
		Reason:  "JobFailed",
		Message: message,
	})
	return ctx.UpdateStatus(ctx.Obj)
}

func (c *BackupActor) completeBackup(ctx *recon.Context[*v1alpha1.BackupJob], backup *v1alpha1.Backup) error {
	bj := ctx.Obj
	//if err := util.Ignore(apierrors.IsNotFound, ctx.Delete(
	//	&batchv1.Job{ObjectMeta: common.ObjMetaTemplate(bj, bj.Name)},
	//	client.PropagationPolicy(metav1.DeletePropagationBackground),
	//)); err != nil {
	//	return errors.Wrap(err, "error finalize backup job")
	//}
	bj.Status.Backup = backup.Name
	bj.Status.Phase = v1alpha1.JobPhaseCompleted
	meta.SetStatusCondition(&bj.Status.Conditions, metav1.Condition{
		Type:   v1alpha1.JobConditionTypeEnded,
		Status: metav1.ConditionTrue,
		Reason: "JobComplete",
	})
	return ctx.UpdateStatus(bj)
}

func (c *BackupActor) syncJob(ctx *recon.Context[*v1alpha1.BackupJob]) error {
	bj := ctx.Obj
	if bj.Status.Phase == "" {
		bj.Status.Phase = v1alpha1.JobPhasePending
	}
	var moSecret string
	backupCmd := &BackupCommand{
		ExtraArgs: bj.Spec.ExtraArgs,
	}
	if bj.Spec.Source.CNSetRef != nil {
		// backup from an CNSet source
		if bj.Spec.Source.SecretRef == nil {
			return errors.New("bad backup config, secretRef must be set when using cnSetRef as backup source")
		}
		moSecret = *bj.Spec.Source.SecretRef
		cn := &v1alpha1.CNSet{}
		if err := ctx.Get(types.NamespacedName{Namespace: bj.Namespace, Name: *bj.Spec.Source.CNSetRef}, cn); err != nil {
			return errors.Wrap(err, "error get cnset")
		}
		if cn.Status.Host == "" {
			return errors.New("cnset host is not set")
		}
		backupCmd.Host = cn.Status.Host
		backupCmd.Port = cn.Status.Port
	} else {
		// backup from an MatrixOneCluster source
		if bj.Spec.Source.ClusterRef == nil {
			return errors.New("bad backup config, either cnSetRef or clusterRef must be set")
		}
		mo := &v1alpha1.MatrixOneCluster{}
		if err := ctx.Get(types.NamespacedName{Namespace: bj.Namespace, Name: *bj.Spec.Source.ClusterRef}, mo); err != nil {
			return errors.Wrap(err, "error get MO")
		}
		if mo.Status.Host == "" {
			return errors.New("MO host is not set")
		}
		if mo.Status.CredentialRef == nil {
			return errors.New("MO credential is not initialized")
		}
		backupCmd.Host = mo.Status.Host
		backupCmd.Port = mo.Status.Port
		moSecret = mo.Status.CredentialRef.Name
	}
	backupCmd.S3.Endpoint = bj.Spec.Target.S3.Endpoint
	backupCmd.S3.Type = string(bj.Spec.Target.S3.GetProviderType())
	parts := strings.SplitN(bj.Spec.Target.S3.Path, "/", 2)
	backupCmd.S3.Bucket = parts[0]
	if len(parts) > 1 {
		backupCmd.S3.Path = parts[1]
	}
	optionalS3Secret := bj.Spec.Target.S3.SecretRef
	if optionalS3Secret != nil {
		backupCmd.S3.ReadEnvSecret = true
	}
	binaryImage := c.backupImage
	if bj.Spec.BinaryImage != "" {
		binaryImage = bj.Spec.BinaryImage
	}
	job := buildJob(bj, binaryImage, backupCmd.String(), func(c *corev1.Container) {
		c.Env = []corev1.EnvVar{{
			Name: MOUserEnvKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: moSecret},
					Key:                  "username",
				},
			},
		}, {
			Name: MOPasswordEnvKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: moSecret},
					Key:                  "password",
				},
			},
		}}
		if optionalS3Secret != nil {
			for _, key := range []string{common.AWSAccessKeyID, common.AWSSecretAccessKey} {
				c.Env = util.UpsertByKey(c.Env, corev1.EnvVar{Name: key, ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: *optionalS3Secret,
						Key:                  key,
					},
				}}, util.EnvVarKey)
			}
		}
	})
	svc := buildSvc(bj)
	if err := util.Ignore(apierrors.IsAlreadyExists, ctx.CreateOwned(job)); err != nil {
		return errors.Wrap(err, "error ensure job")
	}
	if err := util.Ignore(apierrors.IsAlreadyExists, ctx.CreateOwned(svc)); err != nil {
		return errors.Wrap(err, "error ensure service")
	}
	// mark as running
	bj.Status.Phase = v1alpha1.JobPhaseRunning
	meta.SetStatusCondition(&bj.Status.Conditions, metav1.Condition{
		Type:   v1alpha1.JobConditionTypeEnded,
		Status: metav1.ConditionFalse,
		Reason: "JobRunning",
	})
	return ctx.UpdateStatus(bj)
}

func (c *BackupActor) Finalize(ctx *recon.Context[*v1alpha1.BackupJob]) (bool, error) {
	bj := ctx.Obj
	err := ctx.Delete(&batchv1.Job{ObjectMeta: common.ObjMetaTemplate(bj, bj.Name)}, client.PropagationPolicy(metav1.DeletePropagationBackground))
	if err == nil {
		// check next time
		return false, nil
	}
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	return false, errors.Wrap(err, "error delete job")
}

func (c *BackupActor) Reconcile(mgr manager.Manager) error {
	err := recon.Setup[*v1alpha1.BackupJob](&v1alpha1.BackupJob{}, "backupjob", mgr, c, recon.WithBuildFn(func(b *builder.Builder) {
		b.Owns(&batchv1.Job{})
	}))
	if err != nil {
		return err
	}
	return nil
}
