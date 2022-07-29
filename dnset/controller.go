// Copyright 2022 Matrix Origin
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

package controllers

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	BootstrapAnnoKey          = "dnset.matrixorigin.io/bootstrap"
	ReasonNoEnoughReadyStores = "NoEnoughReadyStores"
)

type DNSetActor struct{}

var _ recon.Actor[*v1alpha1.DNSet] = &DNSetActor{}

type DNController struct {
	*DNSetActor
	targetNamespacedName types.NamespacedName
	sts                  *kruisev1alpha1.StatefulSet
}

func (d *DNSetActor) with(sts *kruisev1alpha1.StatefulSet) *DNController {
	return &DNController{DNSetActor: d, sts: sts}
}

func (d *DNSetActor) Observe(ctx *recon.Context[*v1alpha1.DNSet]) (recon.Action[*v1alpha1.DNSet], error) {
	obj := ctx.Obj
	dsts := &kruisev1alpha1.StatefulSet{}

	err, foundSts := util.IsFound(ctx.Get(client.ObjectKey{Namespace: obj.Namespace, Name: StsName(obj)}, dsts))
	if err != nil {
		return nil, errors.Wrap(err, "get dn service statefulset")
	}

	if !foundSts {
		return d.Create, nil
	}

	podList := &corev1.PodList{}
	err = ctx.List(podList, client.InNamespace(obj.Namespace), client.MatchingLabels{common.SubResourceLabels(obj)})
	if err != nil {
		return nil, errors.Wrap(err, "list dn service pods")
	}

	colloctDNStoreStatus(obj, podList.Items)
	if len(obj.Status.AvailableStores) >= int(obj.Spec.Replicas) {
		obj.Status.SetCondition(metav1.Condition{
			Type:   v1alpha1.ConditionTypeReady,
			Status: metav1.ConditionTrue,
		})
	} else {
		obj.Status.SetCondition(metav1.Condition{
			Type:   v1alpha1.ConditionTypeReady,
			Status: metav1.ConditionFalse,
			Reason: ReasonNoEnoughReadyStores,
		})
	}

	return nil, nil
}

func (d *DNSetActor) Create(ctx *recon.Context[*v1alpha1.DNSet]) error {

	return nil
}

func (d *DNSetActor) Finalize(ctx *recon.Context[*v1alpha1.DNSet]) (bool, error) {
	return false, nil
}

func StsName(ls *v1alpha1.DNSet) string {
	return ls.Name
}
