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

package cnlabel

import (
	"context"
	"encoding/json"
	"fmt"
	obs "github.com/matrixorigin/controller-runtime/pkg/observer"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/hacli"
	"github.com/matrixorigin/matrixone/pkg/pb/logservice"
	"github.com/matrixorigin/matrixone/pkg/pb/metadata"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"
)

const retryInterval = 15 * time.Second

type Controller struct {
	clientMgr *hacli.HAKeeperClientManager
}

func NewController(mgr *hacli.HAKeeperClientManager) *Controller {
	return &Controller{clientMgr: mgr}
}

var _ obs.Observer[*corev1.Pod] = &Controller{}

func (c *Controller) Observe(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	if component, ok := pod.Labels[common.ComponentLabelKey]; !ok || component != "CNSet" {
		ctx.Log.V(4).Info("pod is not a CN pod, skip", zap.String("namespace", pod.Namespace), zap.String("name", pod.Name))
		return nil
	}
	cnName, ok := pod.Labels[common.InstanceLabelKey]
	if !ok || cnName == "" {
		return errors.Errorf("cannot find CNSet for CN pod %s/%s, instance label is empty", pod.Namespace, pod.Name)
	}
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	labelStr, ok := pod.Annotations[common.CNLabelAnnotation]
	if !ok {
		// no label to sync
		return nil
	}
	var cnLabels []v1alpha1.CNLabel
	err := json.Unmarshal([]byte(labelStr), &cnLabels)
	if err != nil {
		return errors.Wrap(err, "unmarshal CNLabels")
	}
	cn := &v1alpha1.CNSet{}
	err = ctx.Get(types.NamespacedName{Namespace: pod.Namespace, Name: cnName}, cn)
	if err != nil {
		return errors.Wrap(err, "get CNSet")
	}
	// TODO: consider edge cluster federation scenario
	if cn.Deps.LogSet == nil {
		return errors.Wrapf(err, "cannot get logset of CN pod %s/%s, logset dep is nil", pod.Namespace, pod.Name)
	}
	ls := &v1alpha1.LogSet{}
	// refresh logset status
	if err := ctx.Get(client.ObjectKeyFromObject(cn.Deps.LogSet), ls); err != nil {
		return errors.Wrap(err, "error get logset")
	}
	if !recon.IsReady(ls) {
		return recon.ErrReSync(fmt.Sprintf("logset is not ready for Pod %s, cannot update CN labels", pod.Name), retryInterval)
	}
	haClient, err := c.clientMgr.GetClient(ls)
	if err != nil {
		return errors.Wrap(err, "get HAKeeper client")
	}

	id, err := v1alpha1.GetCNPodUUID(pod)
	if err != nil {
		return errors.Wrap(err, "get cn pod UUID")
	}
	updateCtx, cancel := context.WithTimeout(context.Background(), hacli.HAKeeperTimeout)
	defer cancel()
	err = haClient.UpdateCNLabel(updateCtx, logservice.CNStoreLabel{
		UUID:   id,
		Labels: toStoreLabels(cnLabels),
	})
	if err != nil {
		ctx.Log.Error(err, "update CN label failed")
		return recon.ErrReSync("update cn label failed", retryInterval)
	}
	ctx.Log.Info("successfully update CN labels")
	return nil
}

func toStoreLabels(labels []v1alpha1.CNLabel) map[string]metadata.LabelList {
	lm := make(map[string]metadata.LabelList, len(labels))
	for _, l := range labels {
		lm[l.Key] = metadata.LabelList{
			Labels: l.Values,
		}
	}
	return lm
}

func (c *Controller) Reconcile(mgr manager.Manager) error {
	return obs.Setup[*corev1.Pod](&corev1.Pod{}, "cnlabels", mgr, c,
		recon.SkipStatusSync(),
		recon.WithBuildFn(func(b *builder.Builder) {
			b.WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return false
				}
				if pod.Labels == nil {
					return false
				}
				if component, ok := pod.Labels[common.ComponentLabelKey]; !ok || component != "CNSet" {
					return false
				}
				return true
			}))
		}),
	)
}
