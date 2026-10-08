// Copyright 2026 Matrix Origin
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

package cnstore

import (
	"crypto/sha256"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-errors/errors"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/openkruise/kruise-api/apps/pub"
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var errUpgradeOwnerObservationPending = stderrors.New("CN upgrade CloneSet observation is pending")

func (a *drainAttempt) identityHash() string {
	base := drainAttemptID(a.PodUID, a.CNUUID, a.ContainerID, a.ContainerStartedAt, a.DrainStartedAt, a.Lifecycle)
	if a.Lifecycle != drainLifecycleUpdate {
		return base
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%d", base,
		a.CloneSetUID, a.SourceRevision, a.TargetRevision, a.SourceImageID, a.SourceRestartCount))))
}

func (a *drainAttempt) validUpgradeIdentity() bool {
	if a.Lifecycle != drainLifecycleUpdate {
		return a.CloneSetUID == "" && a.SourceRevision == "" && a.TargetRevision == "" && a.SourceImageID == ""
	}
	return a.CloneSetUID != "" && a.SourceRevision != "" && a.TargetRevision != "" &&
		!revisionLabelMatches(a.SourceRevision, a.TargetRevision) && a.SourceImageID != ""
}

// CloneSet uses the Kubernetes revision label, in either full or short form
// depending on CloneSetShortHash. The update-state record uses the full name.
func revisionLabelMatches(label, revision string) bool {
	if label == "" || revision == "" {
		return false
	}
	return label == revision || label == revision[strings.LastIndex(revision, "-")+1:]
}

func (c *withCNSet) upgradeOwner(ctx *recon.Context[*corev1.Pod], pod *corev1.Pod) (*kruise.CloneSet, error) {
	owner := metav1.GetControllerOf(pod)
	if owner == nil || owner.Kind != "CloneSet" || owner.APIVersion != kruise.SchemeGroupVersion.String() || owner.UID == "" || c.apiReader == nil {
		return nil, errors.New("CN upgrade CloneSet identity is unavailable")
	}
	cs := &kruise.CloneSet{}
	if err := c.apiReader.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: owner.Name}, cs); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("%w: %v", errUpgradeOwnerObservationPending, err)
		}
		return nil, err
	}
	if cs.UID != owner.UID || !cs.DeletionTimestamp.IsZero() {
		return nil, errors.New("CN upgrade CloneSet identity changed")
	}
	if cs.Status.ObservedGeneration != cs.Generation {
		return nil, errUpgradeOwnerObservationPending
	}
	return cs, nil
}

func (c *withCNSet) bindUpgrade(ctx *recon.Context[*corev1.Pod], pod *corev1.Pod, a *drainAttempt) error {
	cs, err := c.upgradeOwner(ctx, pod)
	if err != nil {
		return err
	}
	a.CloneSetUID, a.SourceRevision, a.TargetRevision = string(cs.UID), pod.Labels[appsv1.ControllerRevisionHashLabelKey], cs.Status.UpdateRevision
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == v1alpha1.ContainerMain {
			a.SourceImageID = status.ImageID
			a.SourceRestartCount = status.RestartCount
		}
	}
	if !a.validUpgradeIdentity() {
		return errors.New("CN upgrade source and target proof is incomplete")
	}
	a.AttemptID = a.identityHash()
	return nil
}

// recoverCompletedUpgrade never restores traffic in the same reconcile that
// clears the proof. Protection is re-established first; admission remains the
// normal controller's responsibility on the following observation round.
func (c *withCNSet) recoverCompletedUpgrade(ctx *recon.Context[*corev1.Pod], pod *corev1.Pod, a *drainAttempt) error {
	// Re-arm deletion protection even when the subsequent provenance check
	// fails. The update hook has already been released for this attempt.
	if !controllerutil.ContainsFinalizer(pod, common.CNDrainingFinalizer) {
		controllerutil.AddFinalizer(pod, common.CNDrainingFinalizer)
		if err := ctx.Client.Update(ctx, pod); err != nil {
			return err
		}
		return recon.ErrReSync("CN upgrade protection restored", retryInterval)
	}
	if string(pod.UID) != a.PodUID || v1alpha1.GetCNPodUUID(pod) != a.CNUUID || !pod.DeletionTimestamp.IsZero() {
		return c.requireRecovery(ctx, "CN upgrade instance changed")
	}
	cs, err := c.upgradeOwner(ctx, pod)
	if stderrors.Is(err, errUpgradeOwnerObservationPending) {
		return drainBlocked(ctx, err.Error())
	}
	if err != nil || string(cs.UID) != a.CloneSetUID || cs.Status.UpdateRevision != a.TargetRevision {
		return c.requireRecovery(ctx, "CN upgrade owner or target changed")
	}
	lifecycle := pub.LifecycleStateType(pod.Labels[pub.LifecycleStateKey])
	if lifecycle == pub.LifecycleStateUpdating {
		return drainBlocked(ctx, "CN authorized upgrade is still updating")
	}
	if lifecycle != pub.LifecycleStateUpdated && lifecycle != pub.LifecycleStateNormal {
		return c.requireRecovery(ctx, "CN upgrade lifecycle is not a completed update")
	}
	var state pub.InPlaceUpdateState
	raw, ok := pub.GetInPlaceUpdateState(pod)
	start, parseErr := time.Parse(time.RFC3339Nano, a.DrainStartedAt)
	if !ok || json.Unmarshal([]byte(raw), &state) != nil || parseErr != nil || state.Revision != a.TargetRevision ||
		!revisionLabelMatches(pod.Labels[appsv1.ControllerRevisionHashLabelKey], a.TargetRevision) || state.UpdateTimestamp.Time.Before(start) ||
		state.LastContainerStatuses[v1alpha1.ContainerMain].ImageID != a.SourceImageID ||
		len(state.NextContainerImages) != 0 || len(state.NextContainerRefMetadata) != 0 || state.PreCheckBeforeNext != nil {
		return c.requireRecovery(ctx, "CN upgrade completion provenance is invalid")
	}
	id, started, running := runningContainerIdentity(pod)
	if !running || id == a.ContainerID || started == a.ContainerStartedAt {
		return c.requireRecovery(ctx, "CN upgrade has no new main container")
	}
	newStart, err := time.Parse(time.RFC3339Nano, started)
	if err != nil || newStart.Before(state.UpdateTimestamp.Time) {
		return c.requireRecovery(ctx, "CN main container predates authorized update")
	}
	ready := false
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == v1alpha1.ContainerMain {
			if status.RestartCount != a.SourceRestartCount+1 {
				return c.requireRecovery(ctx, "CN upgrade restart history is not one authorized restart")
			}
			ready = status.Ready && status.ImageID != ""
		}
	}
	updateReady := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == pub.InPlaceUpdateReady {
			updateReady = condition.Status == corev1.ConditionTrue
		}
	}
	score, err := common.GetStoreScore(pod)
	if !ready || !updateReady || err != nil || score.StartedTime == nil || !score.StartedTime.Equal(newStart) ||
		!score.SessionObserved || !score.PipelineObserved || !score.ReplicaObserved {
		return drainBlocked(ctx, "CN upgraded main container lacks current health observation")
	}
	return ctx.Patch(pod, func() error {
		delete(pod.Annotations, drainAttemptAnno)
		delete(pod.Annotations, v1alpha1.StoreDrainingStartAnno)
		delete(pod.Annotations, LockRestartSet)
		return nil
	})
}
