// Copyright 2025 Matrix Origin
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
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-errors/errors"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone/pkg/pb/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CNStateAnno = "matrixorigin.io/cn-state"

	CNDrainingFinalizer = "matrixorigin.io/cn-draining"

	CNStoreReadiness corev1.PodConditionType = "matrixorigin.io/cn-store"

	ReclaimedAt = "matrixorigin.io/reclaimed-at"

	SemanticVersionAnno = "matrixorigin.io/semantic-version"
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
		return nil, errors.WrapPrefix(err, "error get logset", 0)
	}
	return ls, nil
}

// ResolveCNSet resolves the CNSet of an CN Pod
func ResolveCNSet(cli recon.KubeClient, pod *corev1.Pod) (*v1alpha1.CNSet, error) {
	owner, err := ResolveOwner(cli, pod)
	if err != nil {
		return nil, errors.WrapPrefix(err, "error resolve CNSet", 0)
	}
	cnSet, ok := owner.(*v1alpha1.CNSet)
	if !ok {
		return nil, fmt.Errorf("pod is not a CN Pod")
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
		return nil, errors.WrapPrefix(err, "error get owner set", 0)
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

type StoreScore struct {
	SessionCount  int `json:"sessionCount"`
	PipelineCount int `json:"pipelineCount"`
	ReplicaCount  int `json:"replicaCount"`

	StartedTime *time.Time `json:"startedTime,omitempty"`
}

func (s *StoreScore) GenDeletionCost() int {
	return s.SessionCount
}

func (s *StoreScore) IsSafeToReclaim() bool {
	return s.SessionCount == 0 && s.PipelineCount == 0 && s.ReplicaCount == 0
}

func (s *StoreScore) Restarted(startedTime *time.Time) {
	s.SessionCount = 0
	s.PipelineCount = 0
	s.ReplicaCount = 0
	s.StartedTime = startedTime
}

// GetStoreScore get the store connection count from Pod anno
func GetStoreScore(pod *corev1.Pod) (*StoreScore, error) {
	connectionStr, ok := pod.Annotations[v1alpha1.StoreScoreAnno]
	if !ok {
		return nil, errors.Errorf("cannot find connection count for CN pod %s/%s, connection annotation is empty", pod.Namespace, pod.Name)
	}
	s := &StoreScore{}
	if len(connectionStr) == 0 {
		return s, nil
	}
	if err := json.Unmarshal([]byte(connectionStr), s); err != nil {
		// fallback to old format
		count, atoiErr := strconv.Atoi(connectionStr)
		if atoiErr != nil {
			return nil, errors.WrapPrefix(err, "error parsing connection anno", 0)
		}
		s.SessionCount = count
		return s, nil
	}
	return s, nil
}

// SetStoreScore set the store connection info to Pod anno
func SetStoreScore(pod *corev1.Pod, s *StoreScore) error {
	b, err := json.Marshal(s)
	if err != nil {
		return errors.WrapPrefix(err, "error marshal connection info", 0)
	}
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations[v1alpha1.StoreScoreAnno] = string(b)
	return nil
}

// NeedUpdateImage checks if the pod needs to update image
func NeedUpdateImage(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	current := map[string]string{}
	for _, c := range pod.Status.ContainerStatuses {
		current[c.Name] = c.Image
	}
	for _, c := range pod.Spec.Containers {
		if current[c.Name] == "" {
			return true
		}
		if c.Image != current[c.Name] {
			return true
		}
	}
	return false
}

// GetCNStartedTime get the CNStarted Time
func GetCNStartedTime(pod *corev1.Pod) *time.Time {
	for _, c := range pod.Status.ContainerStatuses {
		if c.Name == v1alpha1.ContainerMain {
			if c.State.Running != nil {
				return &c.State.Running.StartedAt.Time
			}
		}
	}
	return nil
}
