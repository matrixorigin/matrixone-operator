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

package common

import (
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone/pkg/pb/metadata"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

const (
	CNStateAnno = "matrixorigin.io/cn-state"

	CNDrainingFinalizer = "matrixorigin.io/cn-draining"

	CNStoreReadiness corev1.PodConditionType = "matrixorigin.io/cn-store"

	ReclaimedAt = "matrixorigin.io/reclaimed-at"
)

func AddReadinessGate(podSpec *corev1.PodSpec, ct corev1.PodConditionType) {
	for _, r := range podSpec.ReadinessGates {
		if r.ConditionType == ct {
			return
		}
	}
	podSpec.ReadinessGates = append(podSpec.ReadinessGates, corev1.PodReadinessGate{
		ConditionType: ct,
	})
}

func GetReadinessCondition(pod *corev1.Pod, conditionType corev1.PodConditionType) *corev1.PodCondition {
	if pod == nil {
		return nil
	}
	for i := range pod.Status.Conditions {
		c := &pod.Status.Conditions[i]
		if c.Type == conditionType {
			return c
		}
	}
	return nil
}

func NewCNReadinessCondition(status corev1.ConditionStatus, msg string) corev1.PodCondition {
	return corev1.PodCondition{
		Type:               CNStoreReadiness,
		Message:            msg,
		Status:             status,
		LastTransitionTime: metav1.Now(),
	}
}

type objectWithDependency interface {
	client.Object
	recon.Dependant
}

func ResolveLogSet(cli recon.KubeClient, cs *v1alpha1.CNSet) (*v1alpha1.LogSet, error) {
	if cs.Deps.LogSet == nil {
		return nil, errors.Errorf("cannot get logset of CNSet %s/%s, logset dep is nil", cs.Namespace, cs.Name)
	}
	ls := &v1alpha1.LogSet{}
	// refresh logset status
	if err := cli.Get(client.ObjectKeyFromObject(cs.Deps.LogSet), ls); err != nil {
		return nil, errors.Wrap(err, "error get logset")
	}
	return ls, nil
}

// ResolveCNSet resoles the CNSet of an CN Pod
func ResolveCNSet(cli recon.KubeClient, pod *corev1.Pod) (*v1alpha1.CNSet, error) {
	owner, err := ResolveOwner(cli, pod)
	if err != nil {
		return nil, errors.Wrap(err, "error resolve CNSet")
	}
	cnSet, ok := owner.(*v1alpha1.CNSet)
	if !ok {
		return nil, errors.Wrap(err, "pod is not a CN Pod")
	}
	return cnSet, nil
}

// ResolveOwner resolves the owner set of an MO Pod
func ResolveOwner(cli recon.KubeClient, pod *corev1.Pod) (client.Object, error) {
	comp, ok := pod.Labels[ComponentLabelKey]
	if !ok {
		return nil, errors.New("cannot resolve logset of non-mo pod")
	}
	instanceName, ok := pod.Labels[InstanceLabelKey]
	if !ok || instanceName == "" {
		return nil, errors.Errorf("cannot find isstance name for pod %s/%s, instance label is empty", pod.Namespace, pod.Name)
	}

	var o client.Object
	switch comp {
	case "CNSet":
		o = &v1alpha1.CNSet{}
	case "DNSet":
		o = &v1alpha1.DNSet{}
	case "LogSet":
		o = &v1alpha1.LogSet{}
	case "ProxySet":
		o = &v1alpha1.ProxySet{}
	default:
		return nil, errors.Errorf("unknown component %s", comp)
	}

	if err := cli.Get(types.NamespacedName{Namespace: pod.Namespace, Name: instanceName}, o); err != nil {
		return nil, errors.Wrap(err, "error get owner set")
	}
	return o, nil
}

// ToStoreLabels transform a list of CNLabel to CNStore Label
func ToStoreLabels(labels []v1alpha1.CNLabel) map[string]metadata.LabelList {
	lm := make(map[string]metadata.LabelList, len(labels))
	for _, l := range labels {
		lm[l.Key] = metadata.LabelList{
			Labels: l.Values,
		}
	}
	return lm
}

// GetStoreConnection get the store connection count from Pod anno
func GetStoreConnection(pod *corev1.Pod) (int, error) {
	connectionStr, ok := pod.Annotations[v1alpha1.StoreConnectionAnno]
	if !ok {
		return 0, errors.Errorf("cannot find connection count for CN pod %s/%s, connection annotation is empty", pod.Namespace, pod.Name)
	}
	count, err := strconv.Atoi(connectionStr)
	if err != nil {
		return 0, errors.Wrap(err, "error parsing connection count")
	}
	return count, nil
}
