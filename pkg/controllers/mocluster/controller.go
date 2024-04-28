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

package mocluster

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/go-errors/errors"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/mosql"
	"github.com/matrixorigin/matrixone-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"time"
)

const (
	RestoreCompleteAnno = "matrixorigin.io/restore-complete"
)

const (
	resyncAfter = 15 * time.Second

	usernameKey = "username"
	passwordKey = "password"
	accountKey  = "account"
	roleKey     = "role"

	maxUnavailablePod = 1

	defaultHKDataPath = "hk_data"
)

var _ recon.Actor[*v1alpha1.MatrixOneCluster] = &MatrixOneClusterActor{}

type MatrixOneClusterActor struct{}

func (r *MatrixOneClusterActor) Observe(ctx *recon.Context[*v1alpha1.MatrixOneCluster]) (recon.Action[*v1alpha1.MatrixOneCluster], error) {
	mo := ctx.Obj
	if mo.Spec.RestoreFrom != nil && mo.Annotations[RestoreCompleteAnno] == "" {
		// do restore
		backup := &v1alpha1.Backup{}
		err := ctx.Get(types.NamespacedName{Name: *mo.Spec.RestoreFrom}, backup)
		if err != nil {
			return nil, errors.WrapPrefix(err, "error get backup", 0)
		}
		restore := &v1alpha1.RestoreJob{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: mo.Namespace,
				Name:      fmt.Sprintf("restore-%s", mo.Name),
			},
			Spec: v1alpha1.RestoreJobSpec{
				BackupName: backup.Name,
				Target:     mo.Spec.LogService.SharedStorage,
			},
		}
		if err := recon.CreateOwnedOrUpdate(ctx, restore, func() error {
			return nil
		}); err != nil {
			return nil, errors.WrapPrefix(err, "error ensure restore job", 0)
		}
		switch restore.Status.Phase {
		case v1alpha1.JobPhaseFailed:
			mo.Status.SetCondition(metav1.Condition{
				Type:    recon.ConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "RestoreFailed",
				Message: "Restore job failed, recreate the cluster to retry",
			})
			return nil, nil
		case v1alpha1.JobPhaseCompleted:
			if mo.Annotations == nil {
				mo.Annotations = map[string]string{}
			}
			mo.Annotations[RestoreCompleteAnno] = string(metav1.ConditionTrue)
			// proceed
		default:
			return nil, recon.ErrReSync("restore job is not completed", resyncAfter)
		}
	}
	return r.Up(ctx)
}

func (r *MatrixOneClusterActor) Up(ctx *recon.Context[*v1alpha1.MatrixOneCluster]) (recon.Action[*v1alpha1.MatrixOneCluster], error) {
	mo := ctx.Obj
	if err := r.InitRootCredential(ctx); err != nil {
		return nil, errors.WrapPrefix(err, "init cluster credential", 0)
	}

	// sync specs
	ls := &v1alpha1.LogSet{
		ObjectMeta: v1alpha1.LogSetKey(mo),
	}
	dn := &v1alpha1.DNSet{
		ObjectMeta: v1alpha1.DNSetKey(mo),
		Deps:       v1alpha1.DNSetDeps{LogSetRef: ls.AsDependency()},
	}
	_, err := utils.CreateOwnedOrUpdate(ctx, ls, func() error {
		ls.Spec = mo.Spec.LogService
		setPodSetDefault(&ls.Spec.PodSet, mo)
		setOverlay(&ls.Spec.Overlay, mo)
		ls.Spec.Image = mo.LogSetImage()
		if mo.Spec.RestoreFrom != nil {
			ls.Spec.InitialConfig.RestoreFrom = pointer.String(defaultHKDataPath)
		}
		return nil
	})
	if err != nil {
		return nil, errors.WrapPrefix(err, "sync LogSet", 0)
	}
	_, err = utils.CreateOwnedOrUpdate(ctx, dn, func() error {
		dn.Spec = *mo.GetTN()
		setPodSetDefault(&dn.Spec.PodSet, mo)
		setOverlay(&dn.Spec.Overlay, mo)
		dn.Spec.Image = mo.DnSetImage()
		return nil
	})
	if err != nil {
		return nil, errors.WrapPrefix(err, "sync DNSet", 0)
	}

	cnGroups := append([]v1alpha1.CNGroup{}, mo.Spec.CNGroups...)
	// append TP and AP cnset for backward compatibility
	if mo.Spec.TP != nil {
		spec := *mo.Spec.TP
		// for backward compatibility, the TP CN may store UUID in cache volume and check consistency
		if spec.DNSBasedIdentity == nil {
			spec.DNSBasedIdentity = pointer.Bool(false)
		}
		cnGroups = append(cnGroups, v1alpha1.CNGroup{Name: "tp", CNSetSpec: spec})
	}
	if mo.Spec.AP != nil {
		cnGroups = append(cnGroups, v1alpha1.CNGroup{Name: "ap", CNSetSpec: *mo.Spec.AP})
	}
	desiredCNSets := map[string]bool{}
	for _, g := range cnGroups {
		cnSetName := fmt.Sprintf("%s-%s", mo.Name, g.Name)
		desiredCNSets[cnSetName] = true
		tpl := &v1alpha1.CNSet{
			ObjectMeta: common.CNSetKey(mo, cnSetName),
			Deps: v1alpha1.CNSetDeps{
				LogSetRef: ls.AsDependency(),
				DNSet:     &v1alpha1.DNSet{ObjectMeta: v1alpha1.DNSetKey(mo)},
			},
		}
		_, err = utils.CreateOwnedOrUpdate(ctx, tpl, func() error {
			// ensure label for legacy CNSet
			if tpl.Labels == nil {
				tpl.Labels = map[string]string{}
			}
			tpl.Labels[common.MatrixoneClusterLabelKey] = mo.Name
			tpl.Spec = g.CNSetSpec
			if mo.Spec.Proxy != nil {
				if tpl.Spec.Config == nil {
					tpl.Spec.Config = v1alpha1.NewTomlConfig(map[string]interface{}{})
				}
				tpl.Spec.Config.Set([]string{"cn", "frontend", "proxy-enabled"}, true)
			}
			tpl.Spec.Overlay = g.Overlay

			// inherit global policies from MO
			setPodSetDefault(&tpl.Spec.PodSet, mo)
			setOverlay(&tpl.Spec.Overlay, mo)
			// upsert DEFAULT_PASSWORD env
			tpl.Spec.Overlay.Env = util.UpsertByKey(tpl.Spec.Overlay.Env, corev1.EnvVar{
				Name: "DEFAULT_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: mo.Status.CredentialRef.Name},
						Key:                  "password",
					},
				},
			}, func(e corev1.EnvVar) string {
				return e.Name
			})
			tpl.Spec.Image = common.CNSetImage(mo, &g.CNSetSpec)
			return nil
		})
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}
	}

	// GC no longer needed CNSets
	groupStatus := v1alpha1.CNGroupsStatus{DesiredGroups: len(cnGroups)}
	csList := &v1alpha1.CNSetList{}
	cnSelector := map[string]string{common.MatrixoneClusterLabelKey: mo.Name}
	if err := ctx.List(csList, client.InNamespace(mo.Namespace), client.MatchingLabels(cnSelector)); err != nil {
		return nil, errors.WrapPrefix(err, "error list current CNSets of the cluster", 0)
	}
	var firstCN *v1alpha1.CNSet
	for i := range csList.Items {
		cnSet := csList.Items[i]
		if !desiredCNSets[cnSet.Name] {
			ctx.Log.V(4).Info("delete CNSet as it is no longer needed", "name", cnSet.Name)
			if err := ctx.Delete(&cnSet); err != nil {
				return nil, errors.WrapPrefix(err, "error delete cnset", 0)
			}
			continue
		}
		if firstCN == nil {
			firstCN = &cnSet
		}
		cngs := v1alpha1.CNGroupStatus{
			Name:   cnSet.Name,
			Host:   fmt.Sprintf("%s.%s", cnSet.Name+"-cn", mo.Namespace),
			Ready:  recon.IsReady(&cnSet),
			Synced: recon.IsSynced(&cnSet),
		}
		if cngs.Ready {
			groupStatus.ReadyGroups++
		}
		if cngs.Synced {
			groupStatus.SyncedGroups++
		}
		groupStatus.Groups = util.UpsertByKey(groupStatus.Groups, cngs, func(v v1alpha1.CNGroupStatus) string {
			return v.Name
		})
	}
	mo.Status.CNGroupStatus = groupStatus

	if mo.Spec.WebUI != nil {
		webui := &v1alpha1.WebUI{
			ObjectMeta: v1alpha1.WebUIKey(mo),
		}
		if err := recon.CreateOwnedOrUpdate(ctx, webui, func() error {
			webui.Spec = *mo.Spec.WebUI
			return nil
		}); err != nil {
			return nil, errors.WrapPrefix(err, "sync webUI", 0)
		}
		mo.Status.Webui = &webui.Status
	}

	if mo.Spec.Proxy != nil {
		proxy := &v1alpha1.ProxySet{
			ObjectMeta: v1alpha1.ProxyKey(mo),
			Deps: v1alpha1.ProxySetDeps{
				LogSetRef: ls.AsDependency(),
			},
		}
		if err := recon.CreateOwnedOrUpdate(ctx, proxy, func() error {
			proxy.Spec = *mo.Spec.Proxy
			setPodSetDefault(&proxy.Spec.PodSet, mo)
			setOverlay(&proxy.Spec.Overlay, mo)
			proxy.Spec.Image = mo.ProxySetImage()
			return nil
		}); err != nil {
			return nil, errors.WrapPrefix(err, "sync proxy", 0)
		}

		mo.Status.Proxy = &proxy.Status
	}

	// collect status
	mo.Status.LogService = &ls.Status
	mo.Status.DN = &dn.Status
	mo.Status.Phase = "NotReady"
	mo.Status.ConditionalStatus.SetCondition(syncedCondition(mo))
	subResourcesReady := readyCondition(mo)
	mo.Status.ConditionalStatus.SetCondition(subResourcesReady)
	if subResourcesReady.Status == metav1.ConditionTrue {
		mo.Status.Phase = "Ready"
	}
	if mo.GetMetricReaderEnabled() {
		if !mo.Status.ClusterMetrics.Initialized && firstCN != nil {
			if err := r.initializeMetricUser(ctx, firstCN.Status.Host); err != nil {
				return nil, errors.WrapPrefix(err, "initialize metric user", 0)
			}
		}
	}

	if recon.IsReady(&mo.Status) {
		if mo.Spec.Proxy != nil {
			if mo.Spec.Proxy == nil {
				return nil, errors.New("proxy status is not set")
			}
			mo.Status.Host = mo.Status.Proxy.Host
			mo.Status.Port = mo.Status.Proxy.Port
		} else {
			if firstCN == nil {
				return nil, errors.New("no CN available")
			}
			mo.Status.Host = firstCN.Status.Host
			mo.Status.Port = firstCN.Status.Port
		}
		return nil, nil
	}
	return nil, recon.ErrReSync("matrixone cluster is not ready", resyncAfter)
}

func (r *MatrixOneClusterActor) initializeMetricUser(ctx *recon.Context[*v1alpha1.MatrixOneCluster], host string) error {
	mo := ctx.Obj
	metricSec, err := r.InitMetricCredential(ctx)
	if err != nil {
		return errors.WrapPrefix(err, "init metric credential", 0)
	}
	sqlcli := mosql.NewClient(fmt.Sprintf("%s:%d", host, 6001), ctx.Client, types.NamespacedName{Namespace: mo.Namespace, Name: mo.Status.CredentialRef.Name})
	if _, err := sqlcli.Query(context.TODO(), fmt.Sprintf("CREATE USER IF NOT EXISTS `%s` identified by '%s'", metricSec.Data[usernameKey], metricSec.Data[passwordKey])); err != nil {
		return errors.WrapPrefix(err, "create operator user", 0)
	}
	role := metricSec.Data[roleKey]
	if _, err := sqlcli.Query(context.TODO(), fmt.Sprintf("CREATE ROLE IF NOT EXISTS `%s`", role)); err != nil {
		return errors.WrapPrefix(err, "create metric role", 0)
	}
	if _, err := sqlcli.Query(context.TODO(), fmt.Sprintf("GRANT connect ON ACCOUNT * TO `%s`", role)); err != nil {
		return errors.WrapPrefix(err, "grant permission", 0)
	}
	if _, err := sqlcli.Query(context.TODO(), fmt.Sprintf("GRANT select ON TABLE system_metrics.* TO %s", role)); err != nil {
		return errors.WrapPrefix(err, "grant permission", 0)
	}
	if _, err := sqlcli.Query(context.TODO(), fmt.Sprintf("GRANT `%s` TO `%s`", role, metricSec.Data[usernameKey])); err != nil {
		return errors.WrapPrefix(err, "grant permission", 0)
	}
	mo.Status.ClusterMetrics.Initialized = true
	return ctx.UpdateStatus(mo)
}

func setPodSetDefault(ps *v1alpha1.PodSet, mo *v1alpha1.MatrixOneCluster) {
	if ps.NodeSelector == nil {
		ps.NodeSelector = mo.Spec.NodeSelector
	}
	if ps.TopologyEvenSpread == nil {
		ps.TopologyEvenSpread = mo.Spec.TopologyEvenSpread
	}
	if ps.SemanticVersion == nil {
		ps.SemanticVersion = mo.Spec.SemanticVersion
	}
}

func setOverlay(o **v1alpha1.Overlay, mo *v1alpha1.MatrixOneCluster) {
	if *o == nil {
		*o = &v1alpha1.Overlay{}
	}
	if (*o).ImagePullPolicy == nil && mo.Spec.ImagePullPolicy != nil {
		(*o).ImagePullPolicy = mo.Spec.ImagePullPolicy
	}
	if (*o).PodLabels == nil {
		(*o).PodLabels = map[string]string{}
	}
	(*o).PodLabels[common.MatrixoneClusterLabelKey] = mo.Name
}

// InitRootCredential init the MO cluster root credential
func (r *MatrixOneClusterActor) InitRootCredential(ctx *recon.Context[*v1alpha1.MatrixOneCluster]) error {
	if ctx.Obj.Status.CredentialRef != nil {
		return nil
	}
	// 1. generate root secret
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.Obj.Namespace,
			Name:      credentialName(ctx.Obj),
		},
		StringData: map[string]string{
			// TODO: avoid using hardcoded username password
			usernameKey: "dump",
			passwordKey: "111",
		},
	}
	if err := ctx.CreateOwned(sec); err != nil {
		return err
	}

	// 2. update the status
	ctx.Obj.Status.CredentialRef = &corev1.LocalObjectReference{Name: sec.Name}
	return ctx.UpdateStatus(ctx.Obj)
}

// InitMetricCredential init the MO cluster metric credential
func (r *MatrixOneClusterActor) InitMetricCredential(ctx *recon.Context[*v1alpha1.MatrixOneCluster]) (*corev1.Secret, error) {
	metricSec := &corev1.Secret{}
	if ctx.Obj.Status.ClusterMetrics.SecretRef != nil {
		if err := ctx.Get(types.NamespacedName{Namespace: ctx.Obj.Namespace, Name: ctx.Obj.Status.ClusterMetrics.SecretRef.Name}, metricSec); err != nil {
			return nil, errors.WrapPrefix(err, "error get metrics secret", 0)
		}
		return metricSec, nil
	}
	// 1. generate metric secret
	metricSec = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.Obj.Namespace,
			Name:      metricCredentialName(ctx.Obj),
		},
		StringData: map[string]string{
			usernameKey: "mo_operator",
			accountKey:  "sys",
			roleKey:     "metric_reader",
			passwordKey: randPassword(12),
		},
	}
	if err := ctx.CreateOwned(metricSec); err != nil {
		return nil, err
	}

	// 2. update the status
	ctx.Obj.Status.ClusterMetrics.SecretRef = &corev1.LocalObjectReference{Name: metricSec.Name}
	return metricSec, ctx.UpdateStatus(ctx.Obj)
}

func readyCondition(mo *v1alpha1.MatrixOneCluster) metav1.Condition {
	c := metav1.Condition{Type: recon.ConditionTypeReady}
	switch {
	case !recon.IsReady(mo.Status.LogService):
		c.Status = metav1.ConditionFalse
		c.Reason = "LogServiceNotReady"
	case !recon.IsReady(mo.Status.DN):
		c.Status = metav1.ConditionFalse
		c.Reason = "DNSetNotReady"
	case !mo.Status.CNGroupStatus.Ready():
		c.Status = metav1.ConditionFalse
		c.Reason = "SomeCNSetsAreNotReady"
	case mo.Spec.Proxy != nil && !recon.IsReady(mo.Status.Proxy):
		c.Status = metav1.ConditionFalse
		c.Reason = "ProxySetNotReady"
	default:
		c.Status = metav1.ConditionTrue
		c.Reason = "AllSetsReady"
	}
	return c
}

func syncedCondition(mo *v1alpha1.MatrixOneCluster) metav1.Condition {
	c := metav1.Condition{Type: recon.ConditionTypeSynced}
	switch {
	case !recon.IsSynced(mo.Status.LogService):
		c.Status = metav1.ConditionFalse
		c.Reason = "LogServiceNotSynced"
	case !recon.IsSynced(mo.Status.DN):
		c.Status = metav1.ConditionFalse
		c.Reason = "DNSetNotSynced"
	case !mo.Status.CNGroupStatus.Synced():
		c.Status = metav1.ConditionFalse
		c.Reason = "SomeCNSetsAreNotSynced"
	case mo.Spec.Proxy != nil && !recon.IsSynced(mo.Status.Proxy):
		c.Status = metav1.ConditionFalse
		c.Reason = "ProxyNotSynced"
	default:
		c.Status = metav1.ConditionTrue
		c.Reason = "AllSetsSynced"
	}
	return c
}

func (r *MatrixOneClusterActor) Finalize(ctx *recon.Context[*v1alpha1.MatrixOneCluster]) (bool, error) {
	mo := ctx.Obj
	err := ctx.Client.DeleteAllOf(ctx, &v1alpha1.CNSet{}, client.InNamespace(mo.Namespace), client.MatchingLabels(
		map[string]string{common.MatrixoneClusterLabelKey: mo.Name},
	))
	if err := util.Ignore(apierrors.IsNotFound, err); err != nil {
		return false, err
	}
	objs := []client.Object{
		&v1alpha1.LogSet{ObjectMeta: v1alpha1.LogSetKey(mo)},
		&v1alpha1.DNSet{ObjectMeta: v1alpha1.DNSetKey(mo)},
		&v1alpha1.WebUI{ObjectMeta: v1alpha1.WebUIKey(mo)},
		&v1alpha1.WebUI{ObjectMeta: v1alpha1.ProxyKey(mo)},
	}
	existAny := false
	for _, obj := range objs {
		exist, err := ctx.Exist(client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return false, err
		}
		if exist {
			if err := util.Ignore(apierrors.IsNotFound, ctx.Delete(obj)); err != nil {
				return false, err
			}
		}
		existAny = existAny || exist
	}
	return !existAny, nil
}

func (r *MatrixOneClusterActor) Reconcile(mgr manager.Manager) error {
	return recon.Setup[*v1alpha1.MatrixOneCluster](&v1alpha1.MatrixOneCluster{}, "matrixonecluster", mgr, r,
		recon.WithBuildFn(func(b *builder.Builder) {
			b.Owns(&v1alpha1.LogSet{}).
				Owns(&v1alpha1.DNSet{}).
				Owns(&v1alpha1.CNSet{}).
				Owns(&v1alpha1.WebUI{}).
				Owns(&v1alpha1.ProxySet{})
		}))
}

func credentialName(mo *v1alpha1.MatrixOneCluster) string {
	return fmt.Sprintf("%s-credential", mo.Name)
}

func metricCredentialName(mo *v1alpha1.MatrixOneCluster) string {
	return fmt.Sprintf("%s-metric", mo.Name)
}

func updatingCondition() metav1.Condition {
	return metav1.Condition{
		Type:   recon.ConditionTypeSynced,
		Status: metav1.ConditionFalse,
		Reason: "Updating",
	}
}

func randPassword(length int) string {
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(randomBytes)[:length]
}
