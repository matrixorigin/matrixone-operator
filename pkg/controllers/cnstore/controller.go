// Copyright 2025-2026 Matrix Origin
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
	"context"
	"crypto/sha256"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"time"

	"github.com/blang/semver/v4"
	"github.com/go-errors/errors"
	gerrors "github.com/go-errors/errors"
	"github.com/go-logr/logr"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/mocli"
	"github.com/matrixorigin/matrixone-operator/pkg/querycli"
	logpb "github.com/matrixorigin/matrixone/pkg/pb/logservice"
	"github.com/matrixorigin/matrixone/pkg/pb/metadata"
	querypb "github.com/matrixorigin/matrixone/pkg/pb/query"
	"github.com/openkruise/kruise-api/apps/pub"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	LockRestartSet = "matrixorigin.io/lock-restart"

	// drainAttemptAnno records the identity of the CN process for which the
	// lock-service drain handshake was started.  LockRestartSet predates this
	// binding and is retained only for cleanup; it is not a safety proof.
	drainAttemptAnno  = "matrixorigin.io/cn-drain-attempt"
	drainRecoveryAnno = "matrixorigin.io/cn-drain-recovery-required"
)

const (
	messageCNCordon             = "CNStoreCordon"
	messageCNPrepareStop        = "CNStorePrepareStop"
	messageCNStoreReady         = "CNStoreReady"
	messageCNStoreNotRegistered = "CNStoreNotRegistered"

	defaultConcurrency = 8

	storeDrainTakesLongDuration = 5 * time.Minute

	diagnosDrainingAnno = "matrixorigin.io/diagnos-draining"
)

const retryInterval = 5 * time.Second
const resyncInterval = 30 * time.Second

type Controller struct {
	clientMgr interface {
		GetClient(*v1alpha1.LogSet) (*mocli.ClientSet, error)
	}
	queryCli     queryClient
	apiReader    client.Reader
	lockClient   lockMigrationClient
	lockIdentity func(context.Context, *corev1.Pod, string, *mocli.ClientSet) (string, error)
	now          func() time.Time
}

func (c *Controller) currentTime() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

type queryClient interface {
	ShowProcessList(context.Context, string) (*querypb.ShowProcessListResponse, error)
	GetPipelineInfo(context.Context, string) (*querypb.GetPipelineInfoResponse, error)
	GetReplicaCount(context.Context, string) (querypb.GetReplicaCountResponse, error)
	GetLockServiceIdentity(context.Context, string) (string, string, error)
}

type withCNSet struct {
	*Controller

	cn *v1alpha1.CNSet
}

type drainAttempt struct {
	Version            int            `json:"version"`
	AttemptID          string         `json:"attemptID"`
	PodUID             string         `json:"podUID"`
	CNUUID             string         `json:"cnUUID"`
	ContainerID        string         `json:"containerID"`
	ContainerStartedAt string         `json:"containerStartedAt"`
	DrainStartedAt     string         `json:"drainStartedAt"`
	Lifecycle          drainLifecycle `json:"lifecycle"`
	Phase              drainPhase     `json:"phase"`
	RestartRequested   bool           `json:"restartRequested"`
	LockServiceID      string         `json:"lockServiceID,omitempty"`
	AllocatorID        string         `json:"allocatorID,omitempty"`
	AllocatorVersion   uint64         `json:"allocatorVersion,omitempty"`
	CloneSetUID        string         `json:"cloneSetUID,omitempty"`
	SourceRevision     string         `json:"sourceRevision,omitempty"`
	TargetRevision     string         `json:"targetRevision,omitempty"`
	SourceImageID      string         `json:"sourceImageID,omitempty"`
	SourceRestartCount int32          `json:"sourceRestartCount,omitempty"`
}

type drainLifecycle string

const (
	drainLifecycleDelete drainLifecycle = "delete"
	drainLifecycleUpdate drainLifecycle = "update"
	drainAttemptVersion                 = 3
	drainPhasePrepared   drainPhase     = "Prepared"
	drainPhaseRequesting drainPhase     = "Requesting"
	drainPhaseRequested  drainPhase     = "Requested"
	drainPhaseCompleted  drainPhase     = "CompletionAuthorized"
	drainPhaseRecovery   drainPhase     = "RecoveryRequired"
)

type drainPhase string

type lockMigrationClient interface {
	BeginDrain(context.Context, string, string) (mocli.DrainProof, error)
	QueryDrain(context.Context, mocli.DrainProof) (bool, error)
	RemainTxnCount(context.Context, string) (int, error)
}

func advanceLockDrain(ctx context.Context, _ string, attempt *drainAttempt, client lockMigrationClient) (safe bool, requested bool, err error) {
	if attempt == nil {
		return false, false, errors.New("CN drain attempt is missing before lock handshake")
	}
	switch attempt.Phase {
	case drainPhaseRequesting:
		if attempt.LockServiceID == "" {
			return false, false, errors.New("CN lock-service instance is missing")
		}
		proof, err := client.BeginDrain(ctx, attempt.LockServiceID, attempt.AttemptID)
		if err != nil {
			return false, false, err
		}
		if proof.ServiceID != attempt.LockServiceID || proof.AttemptID != attempt.AttemptID ||
			proof.AllocatorID == "" || proof.AllocatorVersion == 0 {
			return false, false, errors.New("CN lock-service drain proof does not match the current attempt")
		}
		attempt.AllocatorID, attempt.AllocatorVersion = proof.AllocatorID, proof.AllocatorVersion
		return false, true, nil
	case drainPhaseRequested:
		// A completion query is valid only after the request was durably
		// recorded.  This is intentionally a separate branch so a recovery or
		// malformed phase can never fall through to CanRestartService.
		safe, err = client.QueryDrain(ctx, mocli.DrainProof{ServiceID: attempt.LockServiceID,
			AttemptID: attempt.AttemptID, AllocatorID: attempt.AllocatorID, AllocatorVersion: attempt.AllocatorVersion})
		return safe, false, err
	case drainPhaseRecovery:
		return false, false, errors.New("CN drain attempt requires recovery before lock handshake")
	default:
		return false, false, errors.New("CN drain attempt phase is invalid before lock handshake")
	}
}

func NewController(mgr *mocli.MORPCClientManager, qc *querycli.Client) *Controller {
	return &Controller{clientMgr: mgr, queryCli: qc}
}

var _ recon.Actor[*corev1.Pod] = &Controller{}

func runningContainerIdentity(pod *corev1.Pod) (string, string, bool) {
	if pod == nil {
		return "", "", false
	}
	for i := range pod.Status.ContainerStatuses {
		status := &pod.Status.ContainerStatuses[i]
		if status.Name != v1alpha1.ContainerMain {
			continue
		}
		if status.State.Running == nil || status.ContainerID == "" || status.State.Running.StartedAt.IsZero() {
			return "", "", false
		}
		return status.ContainerID, status.State.Running.StartedAt.UTC().Format(time.RFC3339Nano), true
	}
	return "", "", false
}

func lifecycleForPod(pod *corev1.Pod) (drainLifecycle, bool) {
	if pod == nil {
		return "", false
	}
	switch pod.Labels[pub.LifecycleStateKey] {
	case string(pub.LifecycleStatePreparingDelete):
		return drainLifecycleDelete, true
	case string(pub.LifecycleStatePreparingUpdate):
		return drainLifecycleUpdate, true
	default:
		return "", false
	}
}

func drainAttemptID(podUID, cnUUID, containerID, containerStartedAt, drainStartedAt string, lifecycle drainLifecycle) string {
	seed := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%s", podUID, cnUUID, containerID, containerStartedAt, drainStartedAt, lifecycle)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(seed)))
}

func newDrainAttempt(pod *corev1.Pod, cnUUID string, startTime time.Time, lifecycle drainLifecycle) (*drainAttempt, error) {
	if pod == nil || pod.UID == "" || cnUUID == "" || lifecycle == "" {
		return nil, errors.New("CN drain identity is incomplete")
	}
	containerID, startedAt, ok := runningContainerIdentity(pod)
	if !ok {
		return nil, errors.New("CN drain container identity is unavailable")
	}
	started := startTime.UTC().Format(time.RFC3339Nano)
	return &drainAttempt{
		Version:            drainAttemptVersion,
		AttemptID:          drainAttemptID(string(pod.UID), cnUUID, containerID, startedAt, started, lifecycle),
		PodUID:             string(pod.UID),
		CNUUID:             cnUUID,
		ContainerID:        containerID,
		ContainerStartedAt: startedAt,
		DrainStartedAt:     started,
		Lifecycle:          lifecycle,
		Phase:              drainPhasePrepared,
	}, nil
}

func readDrainAttempt(pod *corev1.Pod) (*drainAttempt, error) {
	if pod == nil || pod.Annotations == nil {
		return nil, nil
	}
	raw, ok := pod.Annotations[drainAttemptAnno]
	if !ok {
		return nil, nil
	}
	attempt := &drainAttempt{}
	if err := json.Unmarshal([]byte(raw), attempt); err != nil {
		return nil, errors.WrapPrefix(err, "parse CN drain attempt", 0)
	}
	if attempt.Version != drainAttemptVersion || attempt.AttemptID == "" ||
		attempt.PodUID == "" || attempt.CNUUID == "" || attempt.ContainerID == "" ||
		attempt.ContainerStartedAt == "" || attempt.DrainStartedAt == "" ||
		(attempt.Lifecycle != drainLifecycleDelete && attempt.Lifecycle != drainLifecycleUpdate) ||
		(attempt.Phase != drainPhasePrepared && attempt.Phase != drainPhaseRequesting &&
			attempt.Phase != drainPhaseRequested && attempt.Phase != drainPhaseCompleted && attempt.Phase != drainPhaseRecovery) ||
		!drainAttemptPhaseConsistent(attempt) ||
		!attempt.validUpgradeIdentity() || attempt.AttemptID != attempt.identityHash() {
		return nil, errors.New("CN drain attempt identity is incomplete")
	}
	return attempt, nil
}

func drainAttemptPhaseConsistent(attempt *drainAttempt) bool {
	if attempt == nil {
		return false
	}
	switch attempt.Phase {
	case drainPhasePrepared:
		return !attempt.RestartRequested && attempt.LockServiceID == "" && attempt.AllocatorID == "" && attempt.AllocatorVersion == 0
	case drainPhaseRequesting:
		return !attempt.RestartRequested && attempt.LockServiceID != "" && attempt.AllocatorID == "" && attempt.AllocatorVersion == 0
	case drainPhaseRequested, drainPhaseCompleted:
		return attempt.RestartRequested && attempt.LockServiceID != "" && attempt.AllocatorID != "" && attempt.AllocatorVersion != 0
	case drainPhaseRecovery:
		// RecoveryRequired may be entered before the RPC was sent or after an
		// ambiguous request; preserve either value as diagnostic evidence.
		return true
	default:
		return false
	}
}

func (a *drainAttempt) matches(current *drainAttempt) bool {
	if a == nil || current == nil {
		return false
	}
	return a.PodUID == current.PodUID &&
		a.AttemptID == current.AttemptID &&
		a.CNUUID == current.CNUUID &&
		a.ContainerID == current.ContainerID &&
		a.ContainerStartedAt == current.ContainerStartedAt &&
		a.DrainStartedAt == current.DrainStartedAt &&
		a.Lifecycle == current.Lifecycle
}

func drainBlocked(ctx *recon.Context[*corev1.Pod], reason string) error {
	ctx.Log.Info("CN drain blocked; deletion protection remains active", "reason", reason)
	return recon.ErrReSync(reason, retryInterval)
}

func (c *withCNSet) requireRecovery(ctx *recon.Context[*corev1.Pod], reason string) error {
	if err := c.persistRecovery(ctx); err != nil {
		ctx.Log.Error(err, "cannot persist CN drain recovery state", "reason", reason)
	}
	return drainBlocked(ctx, reason)
}

func (c *withCNSet) persistRecovery(ctx *recon.Context[*corev1.Pod]) error {
	fresh, err := c.freshPod(ctx)
	if err != nil {
		return err
	}
	if fresh.UID != ctx.Obj.UID {
		return errors.New("CN replaced before recovery write")
	}
	attempt, err := readDrainAttempt(fresh)
	if err != nil || attempt == nil {
		// Preserve malformed or legacy evidence rather than manufacturing a
		// new handshake identity from the current process.
		ctx.Obj = fresh
		return ctx.Patch(fresh, func() error {
			if fresh.Annotations == nil {
				fresh.Annotations = map[string]string{}
			}
			fresh.Annotations[drainRecoveryAnno] = "invalid-or-missing-attempt"
			return nil
		})
	}
	if attempt.Phase == drainPhaseRecovery {
		return nil
	}
	attempt.Phase = drainPhaseRecovery
	payload, err := json.Marshal(attempt)
	if err != nil {
		return err
	}
	ctx.Obj = fresh
	return ctx.Patch(ctx.Obj, func() error {
		if ctx.Obj.Annotations == nil {
			ctx.Obj.Annotations = map[string]string{}
		}
		ctx.Obj.Annotations[drainAttemptAnno] = string(payload)
		return nil
	})
}

func (c *withCNSet) ensureDrainAttempt(ctx *recon.Context[*corev1.Pod], uid string, startTime time.Time, lifecycle drainLifecycle) (*drainAttempt, error) {
	fresh, err := c.freshPod(ctx)
	if err != nil {
		return nil, drainBlocked(ctx, "fresh CN read failed before drain attempt")
	}
	freshUID := v1alpha1.GetCNPodUUID(fresh)
	if fresh.Annotations[drainRecoveryAnno] != "" {
		return nil, drainBlocked(ctx, "CN drain recovery diagnostic requires intervention")
	}
	if freshUID != uid {
		return nil, c.requireRecovery(ctx, "CN identity changed before drain attempt")
	}
	freshLifecycle, ok := lifecycleForPod(fresh)
	if !ok || freshLifecycle != lifecycle {
		return nil, c.requireRecovery(ctx, "CN lifecycle changed before drain attempt")
	}
	current, err := newDrainAttempt(fresh, uid, startTime, lifecycle)
	if err != nil {
		return nil, drainBlocked(ctx, err.Error())
	}
	if lifecycle == drainLifecycleUpdate {
		if err := c.bindUpgrade(ctx, fresh, current); err != nil {
			if stderrors.Is(err, errUpgradeOwnerObservationPending) {
				return nil, drainBlocked(ctx, err.Error())
			}
			return nil, c.requireRecovery(ctx, err.Error())
		}
	}
	previous, err := readDrainAttempt(fresh)
	if err != nil {
		return nil, c.requireRecovery(ctx, "CN drain attempt is invalid; recovery is required")
	}
	if previous == nil {
		if _, legacy := fresh.Annotations[LockRestartSet]; legacy {
			return nil, c.requireRecovery(ctx, "legacy lock-restart marker requires recovery")
		}
		payload, marshalErr := json.Marshal(current)
		if marshalErr != nil {
			return nil, errors.WrapPrefix(marshalErr, "marshal CN drain attempt", 0)
		}
		ctx.Obj = fresh
		if patchErr := ctx.Patch(ctx.Obj, func() error {
			if ctx.Obj.Annotations == nil {
				ctx.Obj.Annotations = map[string]string{}
			}
			ctx.Obj.Annotations[drainAttemptAnno] = string(payload)
			return nil
		}); patchErr != nil {
			return nil, errors.WrapPrefix(patchErr, "record CN drain attempt", 0)
		}
		return nil, drainBlocked(ctx, "record CN drain attempt identity")
	}
	if !previous.matches(current) {
		return nil, c.requireRecovery(ctx, "CN process identity or lifecycle changed; stale drain attempt requires recovery")
	}
	if previous.Phase == drainPhaseRecovery {
		return nil, drainBlocked(ctx, "CN drain attempt requires recovery")
	}
	ctx.Obj = fresh
	return previous, nil
}

func (c *withCNSet) freshPod(ctx *recon.Context[*corev1.Pod]) (*corev1.Pod, error) {
	fresh := &corev1.Pod{}
	key := client.ObjectKeyFromObject(ctx.Obj)
	var err error
	if c.apiReader != nil {
		err = c.apiReader.Get(ctx, key, fresh)
	} else {
		return nil, errors.New("non-cached CN API reader is unavailable")
	}
	if err != nil {
		return nil, err
	}
	return fresh, nil
}

func (c *withCNSet) verifyDrainAttempt(ctx *recon.Context[*corev1.Pod], snapshot *drainAttempt) (*corev1.Pod, error) {
	fresh, err := c.freshPod(ctx)
	if err != nil {
		return nil, drainBlocked(ctx, "fresh CN read failed before drain completion")
	}
	if snapshot == nil || snapshot.Phase != drainPhaseRequested {
		return nil, drainBlocked(ctx, "CN completion query snapshot is missing")
	}
	if _, err := validateDrainSnapshot(fresh, snapshot); err != nil {
		return nil, drainBlocked(ctx, err.Error())
	}
	return fresh, nil
}

// validateDrainSnapshot compares both the persisted request and the actual
// running process. An unchanged annotation alone does not prove identity.
func validateDrainSnapshot(pod *corev1.Pod, expected *drainAttempt) (*drainAttempt, error) {
	current, err := readDrainAttempt(pod)
	if err != nil || expected == nil || current == nil || *current != *expected {
		return nil, errors.New("CN drain request snapshot changed")
	}
	lifecycle, ok := lifecycleForPod(pod)
	containerID, startedAt, running := runningContainerIdentity(pod)
	if !ok || lifecycle != expected.Lifecycle || !running || pod.Annotations[drainRecoveryAnno] != "" ||
		string(pod.UID) != expected.PodUID || v1alpha1.GetCNPodUUID(pod) != expected.CNUUID ||
		containerID != expected.ContainerID || startedAt != expected.ContainerStartedAt ||
		!pod.DeletionTimestamp.IsZero() {
		return nil, errors.New("CN drain actual instance or lifecycle changed")
	}
	if expected.Lifecycle == drainLifecycleUpdate {
		owner := metav1.GetControllerOf(pod)
		if owner == nil || string(owner.UID) != expected.CloneSetUID || owner.Kind != "CloneSet" ||
			pod.Labels[appsv1.ControllerRevisionHashLabelKey] != expected.SourceRevision {
			return nil, errors.New("CN upgrade owner or source revision changed")
		}
	}
	return current, nil
}

func (c *withCNSet) persistDrainPhase(ctx *recon.Context[*corev1.Pod], expected *drainAttempt, phase drainPhase) error {
	return c.persistDrainTransition(ctx, expected, phase, "", mocli.DrainProof{})
}

func (c *withCNSet) persistDrainTransition(ctx *recon.Context[*corev1.Pod], expected *drainAttempt, phase drainPhase, serviceID string, proof mocli.DrainProof) error {
	if expected == nil {
		return errors.New("CN drain attempt is missing before phase update")
	}
	if !((phase == drainPhaseRequesting && (expected.Phase == drainPhasePrepared || expected.Phase == drainPhaseRequesting)) ||
		(phase == drainPhaseRequested && expected.Phase == drainPhaseRequesting) ||
		(phase == drainPhaseCompleted && expected.Phase == drainPhaseRequested)) {
		return errors.New("CN drain phase transition is invalid")
	}
	fresh, err := c.freshPod(ctx)
	if err != nil {
		return errors.WrapPrefix(err, "fresh CN read before phase update", 0)
	}
	current, err := validateDrainSnapshot(fresh, expected)
	if err != nil {
		return err
	}
	current.Phase = phase
	current.RestartRequested = phase == drainPhaseRequested || phase == drainPhaseCompleted
	if phase == drainPhaseRequesting && expected.Phase == drainPhasePrepared {
		if serviceID == "" {
			return errors.New("CN lock-service identity is missing before drain request")
		}
		current.LockServiceID = serviceID
	}
	if phase == drainPhaseRequested {
		if proof.ServiceID != current.LockServiceID || proof.AttemptID != current.AttemptID ||
			proof.AllocatorID == "" || proof.AllocatorVersion == 0 {
			return errors.New("CN lock-service drain proof does not match attempt")
		}
		current.AllocatorID, current.AllocatorVersion = proof.AllocatorID, proof.AllocatorVersion
	}
	payload, err := json.Marshal(current)
	if err != nil {
		return errors.WrapPrefix(err, "marshal CN drain phase", 0)
	}
	ctx.Obj = fresh
	return ctx.Patch(ctx.Obj, func() error {
		if ctx.Obj.Annotations == nil {
			ctx.Obj.Annotations = map[string]string{}
		}
		ctx.Obj.Annotations[drainAttemptAnno] = string(payload)
		delete(ctx.Obj.Annotations, LockRestartSet)
		return nil
	})
}

// OnDeleted delete CNStore and cleanup finalizer on Pod deletion
func (c *Controller) OnDeleted(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	uid := v1alpha1.GetCNPodUUID(pod)
	cnSet, err := common.ResolveCNSet(ctx, pod)
	if err == nil {
		wc := &withCNSet{
			Controller: c,
			cn:         cnSet,
		}
		// clean up CN store if any, note that OnDeleted() and the termination of Pod containers
		// are simultaneous, so the cleanup below is merely a best-effort attempt in extraordinary case, e.g.
		// the Pod is deleted forcefully by a human operator. Normal cleanup must be done before we enter OnDeleted()
		// to avoid zombie CN in HAKeeper.
		ctx.Log.Info("call HAKeeper to remove CN store", "uuid", uid)
		err = wc.withMOClientSet(ctx, func(timeout context.Context, h *mocli.ClientSet) error {
			return h.Client.DeleteCNStore(timeout, logpb.DeleteCNStore{
				StoreID: uid,
			})
		})
		if err != nil {
			return errors.WrapPrefix(err, "error remove CN store", 0)
		}
	} else {
		ctx.Log.Info("error resolve CNSet of the deleted CN, skip", "error", err.Error())
	}
	if err := ctx.Patch(pod, func() error {
		controllerutil.RemoveFinalizer(pod, common.CNDrainingFinalizer)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// OnPreparingUpdate perform actions that should be done on CN preparing stop
func (c *withCNSet) OnPreparingUpdate(ctx *recon.Context[*corev1.Pod]) error {
	if c.cn.Spec.PauseUpdate {
		// PauseUpdate alone does not identify the Pod revision or exclude an
		// envFrom change. Until that proof is available it cannot bypass drain.
		return drainBlocked(ctx, "paused update lacks instance-bound no-restart proof")
	}
	// TODO: should diff with cloneset spec
	// if pod image is not going to be updated, skip draining
	// NB: change envFrom(labels/annotations) will restart container in-place, but we cannot
	// distinguish such case now, CN will be restarted without draining if we introduce envFrom
	// mutation in other modules. E2Es are needed to guard such issue.
	//if !common.NeedUpdateImage(ctx.Obj) {
	//	ctx.Log.Info("skip draining CN store, no image update", "CN", client.ObjectKeyFromObject(ctx.Obj))
	//	return c.completeDraining(ctx)
	//}
	return c.OnPreparingStop(ctx)
}

// OnPreparingStop drains CN connections
func (c *withCNSet) OnPreparingStop(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	uid := v1alpha1.GetCNPodUUID(ctx.Obj)
	lifecycle, ok := lifecycleForPod(pod)
	if !ok {
		return drainBlocked(ctx, "CN lifecycle is not a supported drain state")
	}
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}

	if err := c.patchCNReadiness(ctx, corev1.ConditionFalse, messageCNPrepareStop); err != nil {
		return errors.WrapPrefix(err, "patch pod readiness", 0)
	}
	// A normal restart must never bypass the lock-service handshake. The
	// instance-bound CN identity and TN drain proof establish capability;
	// a version annotation alone cannot establish safe retirement.
	sc := c.cn.Spec.ScalingConfig
	if !sc.GetStoreDrainEnabled() {
		return drainBlocked(ctx, "store drain is disabled; safe CN retirement is unavailable")
	}

	// start draining
	var startTime time.Time
	startTimeStr, ok := pod.Annotations[v1alpha1.StoreDrainingStartAnno]
	if ok {
		parsed, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		startTime = parsed
	} else {
		startTime = c.currentTime()
		if err := ctx.Patch(pod, func() error {
			if pod.Annotations == nil {
				pod.Annotations = map[string]string{}
			}
			pod.Annotations[v1alpha1.StoreDrainingStartAnno] = startTime.Format(time.RFC3339)
			return nil
		}); err != nil {
			return errors.WrapPrefix(err, "error patching store draining start time", 0)
		}
		return drainBlocked(ctx, "record CN drain start time")
	}
	attempt, err := c.ensureDrainAttempt(ctx, uid, startTime, lifecycle)
	if err != nil {
		return err
	}
	if attempt.Phase == drainPhaseCompleted {
		return c.completeDraining(ctx, ctx.Obj.DeepCopy())
	}
	// A drain deadline is an alert threshold, not a terminal state. Keep the
	// protection while observing the same attempt: a remote transaction may
	// release its locks after this deadline without any unsafe force-delete.
	overdue := c.currentTime().Sub(startTime) > sc.GetStoreDrainTimeout()

	var connAndShardMigrated, lockMigrated bool
	err = c.withMOClientSet(ctx, func(timeout context.Context, h *mocli.ClientSet) error {
		var err error
		connAndShardMigrated, err = c.handleConnectionDraining(ctx, uid, timeout, h)
		if err != nil {
			return err
		}
		if c.currentTime().Sub(startTime) < sc.GetMinDelayDuration() {
			return recon.ErrReSync("wait min-delay for CN draining state get propagated", sc.GetMinDelayDuration())
		}
		if !connAndShardMigrated {
			// lock migration should be done after connection get migrated
			return nil
		}
		lockMigrated, err = c.handleLockMigration(ctx, uid, timeout, h, attempt)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if connAndShardMigrated && lockMigrated {
		_, err := c.verifyDrainAttempt(ctx, attempt)
		if err != nil {
			return err
		}
		if err := c.persistDrainPhase(ctx, attempt, drainPhaseCompleted); err != nil {
			return err
		}
		return c.completeDraining(ctx, ctx.Obj.DeepCopy())
	}
	if c.currentTime().Sub(startTime) > storeDrainTakesLongDuration {
		c.diagnosisDraining(ctx, uid)
	}
	if overdue {
		return drainBlocked(ctx, "store draining timeout; continuing safe CN drain observation")
	}
	return recon.ErrReSync("wait for CN store draining", retryInterval)
}

func (c *Controller) diagnosisDraining(ctx *recon.Context[*corev1.Pod], uid string) {
	ctx.Log.Info("store draining takes too long, collect diagnostic info", "uuid", uid)
	pod := ctx.Obj
	if err := ctx.Patch(pod, func() error {
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		pod.Annotations[diagnosDrainingAnno] = "y"
		return nil
	}); err != nil {
		ctx.Log.Error(err, "error patching diagnos draining anno")
	}
}

func (c *withCNSet) handleConnectionDraining(ctx *recon.Context[*corev1.Pod], uid string, timeout context.Context, h *mocli.ClientSet) (bool, error) {
	pod := ctx.Obj
	ctx.Log.Info("set CN store draining", "uuid", uid)
	if err := h.Client.PatchCNStore(timeout, logpb.CNStateLabel{
		UUID:  uid,
		State: metadata.WorkState_Draining,
	}); err != nil {
		return false, errors.WrapPrefix(err, "error set CN state draining", 0)
	}
	storeConnection, err := common.GetStoreScore(pod)
	if err != nil {
		return false, errors.WrapPrefix(err, "error get store connection count", 0)
	}

	if !storeConnection.IsSafeToReclaim() {
		ctx.Log.Info("wait store connection to be drained", "uuid", uid, "connections", storeConnection.SessionCount, "pipelines", storeConnection.PipelineCount, "replicas", storeConnection.ReplicaCount)
	}
	return storeConnection.IsSafeToReclaim(), nil
}

func (c *withCNSet) handleLockMigration(ctx *recon.Context[*corev1.Pod], uid string, timeout context.Context, h *mocli.ClientSet, attempt *drainAttempt) (bool, error) {
	if attempt == nil {
		return false, drainBlocked(ctx, "CN drain attempt is missing before lock handshake")
	}
	if attempt.Phase == drainPhasePrepared || attempt.Phase == drainPhaseRequesting {
		fresh, err := c.freshPod(ctx)
		if err != nil {
			return false, drainBlocked(ctx, "CN identity read failed before lock drain")
		}
		if _, err := validateDrainSnapshot(fresh, attempt); err != nil {
			return false, drainBlocked(ctx, err.Error())
		}
		serviceID, err := c.discoverLockServiceID(timeout, fresh, uid, h)
		if err != nil || (attempt.Phase == drainPhaseRequesting && serviceID != attempt.LockServiceID) {
			return false, drainBlocked(ctx, "CN lock-service instance cannot be confirmed")
		}
		// Persist the intent before the RPC. If the response or the following
		// annotation write is lost, the next reconcile stays fail-closed and
		// repeats SetRestart instead of accepting CanRestart from an old state.
		next := *attempt
		if err := c.persistDrainTransition(ctx, &next, drainPhaseRequesting, serviceID, mocli.DrainProof{}); err != nil {
			return false, errors.Wrap(err, 0)
		}
		next.Phase = drainPhaseRequesting
		next.LockServiceID = serviceID
		attempt = &next
	}
	lockClient := c.lockClient
	if lockClient == nil {
		lockClient = h.LockServiceClient
	}
	safe, requested, err := advanceLockDrain(timeout, uid, attempt, lockClient)
	if err != nil {
		return false, err
	}
	if requested {
		proof := mocli.DrainProof{ServiceID: attempt.LockServiceID, AttemptID: attempt.AttemptID,
			AllocatorID: attempt.AllocatorID, AllocatorVersion: attempt.AllocatorVersion}
		previous := *attempt
		previous.AllocatorID, previous.AllocatorVersion = "", 0
		if err := c.persistDrainTransition(ctx, &previous, drainPhaseRequested, "", proof); err != nil {
			return false, errors.Wrap(err, 0)
		}
		return false, nil
	}
	if attempt.Phase == drainPhaseRecovery {
		return false, drainBlocked(ctx, "CN drain attempt requires recovery")
	}
	if attempt.Phase != drainPhaseRequested {
		return false, drainBlocked(ctx, "CN drain attempt phase is invalid")
	}
	if !safe {
		ctx.Log.Info("cannot restart CN now, check reason", "UID", uid)
		remainTxns, err := lockClient.RemainTxnCount(timeout, attempt.LockServiceID)
		if err != nil {
			ctx.Log.Error(err, "cannot get remaining transactions")
		} else {
			ctx.Log.Info("CN has remaining transactions, cannot restart now", "UID", uid, "remainTxns", remainTxns)
		}
		return false, nil
	}
	fresh, err := c.freshPod(ctx)
	if err != nil {
		return false, drainBlocked(ctx, "CN identity read failed after lock drain")
	}
	if _, err := validateDrainSnapshot(fresh, attempt); err != nil {
		return false, drainBlocked(ctx, err.Error())
	}
	serviceID, err := c.discoverLockServiceID(timeout, fresh, uid, h)
	if err != nil || serviceID != attempt.LockServiceID {
		return false, drainBlocked(ctx, "CN lock-service instance changed after completion query")
	}
	ctx.Log.Info("lock-service migrated, can restart CN now", "UUID", uid)
	return true, nil
}

func (c *withCNSet) discoverLockServiceID(ctx context.Context, pod *corev1.Pod, uid string, h *mocli.ClientSet) (string, error) {
	if c.lockIdentity != nil {
		return c.lockIdentity(ctx, pod, uid, h)
	}
	if h == nil || h.StoreCache == nil || c.queryCli == nil || pod.Status.PodIP == "" {
		return "", errors.New("CN lock-service identity source is unavailable")
	}
	cn, ok := h.StoreCache.GetCN(uid)
	if !ok || cn.QueryAddress == "" {
		return "", errors.New("CN query endpoint is unavailable")
	}
	host, _, err := net.SplitHostPort(cn.QueryAddress)
	if err != nil || host != pod.Status.PodIP {
		return "", errors.New("CN query endpoint does not match current Pod IP")
	}
	cnUUID, serviceID, err := c.queryCli.GetLockServiceIdentity(ctx, cn.QueryAddress)
	if err != nil || cnUUID != uid || len(serviceID) <= len(uid) || serviceID[len(serviceID)-len(uid):] != uid {
		return "", errors.New("CN lock-service identity does not match current CN")
	}
	return serviceID, nil
}

func (c *withCNSet) completeDraining(ctx *recon.Context[*corev1.Pod], verified *corev1.Pod) error {
	if verified == nil {
		return drainBlocked(ctx, "CN completion authorization is missing")
	}
	fresh, err := c.freshPod(ctx)
	if err != nil {
		return errors.WrapPrefix(err, "fresh CN read before finalizer removal", 0)
	}
	if _, ok := lifecycleForPod(fresh); !ok {
		return drainBlocked(ctx, "CN lifecycle changed before finalizer removal")
	}
	if fresh.UID != verified.UID || fresh.ResourceVersion != verified.ResourceVersion {
		return drainBlocked(ctx, "CN identity or resource version changed before finalizer removal")
	}
	attempt, err := readDrainAttempt(verified)
	if err != nil || attempt == nil || attempt.Phase != drainPhaseCompleted {
		return drainBlocked(ctx, "CN completion authorization is missing")
	}
	if _, err := validateDrainSnapshot(fresh, attempt); err != nil {
		return drainBlocked(ctx, err.Error())
	}
	verified = fresh
	direct := verified.Labels[v1alpha1.DirectPodLabel] != ""
	if direct {
		// Keep both the authorization and finalizer until DELETE is accepted.
		// OnDeleted owns finalizer cleanup; a lost response can never cause a
		// second handshake or authorize a same-name replacement.
		uid := verified.UID
		rv := verified.ResourceVersion
		if err := ctx.Client.Delete(ctx, verified, client.Preconditions{UID: &uid, ResourceVersion: &rv}); err != nil {
			return errors.Wrap(err, 0)
		}
		return nil
	}
	controllerutil.RemoveFinalizer(verified, common.CNDrainingFinalizer)
	if err := ctx.Client.Update(ctx, verified); err != nil {
		return errors.WrapPrefix(err, "error removing CN draining finalizer", 0)
	}
	return nil
}

// OnNormal ensure CNStore labels and transit CN store to UP state
func (c *withCNSet) OnNormal(ctx *recon.Context[*corev1.Pod]) error {
	pod, err := c.freshPod(ctx)
	if err != nil {
		return err
	}
	if pod.UID != ctx.Obj.UID {
		return drainBlocked(ctx, "CN replaced before cancellation")
	}
	if _, draining := lifecycleForPod(pod); draining {
		return drainBlocked(ctx, "CN lifecycle changed before cancellation")
	}
	ctx.Obj = pod
	if pod.Annotations[drainRecoveryAnno] != "" {
		return drainBlocked(ctx, "CN recovery diagnostic requires intervention")
	}
	attempt, err := readDrainAttempt(pod)
	if err != nil {
		return drainBlocked(ctx, "CN drain attempt is invalid; recovery is required")
	} else if attempt != nil && attempt.Phase == drainPhaseCompleted && attempt.Lifecycle == drainLifecycleUpdate {
		return c.recoverCompletedUpgrade(ctx, pod, attempt)
	} else if attempt != nil && attempt.Phase != drainPhasePrepared {
		return c.requireRecovery(ctx, "DrainCancellationRequiresRecovery")
	} else if attempt != nil {
		id, started, running := runningContainerIdentity(pod)
		if !running || string(pod.UID) != attempt.PodUID || v1alpha1.GetCNPodUUID(pod) != attempt.CNUUID ||
			id != attempt.ContainerID || started != attempt.ContainerStartedAt {
			return c.requireRecovery(ctx, "CN identity changed before prepared cancellation")
		}
		if err := ctx.Patch(pod, func() error {
			delete(pod.Annotations, drainAttemptAnno)
			delete(pod.Annotations, v1alpha1.StoreDrainingStartAnno)
			delete(pod.Annotations, LockRestartSet)
			return nil
		}); err != nil {
			return errors.WrapPrefix(err, "clear prepared CN drain attempt", 0)
		}
		return recon.ErrReSync("prepared CN drain attempt cleared", retryInterval)
	}
	if _, legacy := pod.Annotations[LockRestartSet]; legacy {
		return drainBlocked(ctx, "legacy lock-restart marker requires recovery")
	}
	if state := pub.LifecycleStateType(pod.Labels[pub.LifecycleStateKey]); state == pub.LifecycleStateUpdating || state == pub.LifecycleStateUpdated {
		return drainBlocked(ctx, "CN upgrade must reach Normal before business admission")
	}

	// ensure finalizers
	if err := ctx.Patch(pod, func() error {
		controllerutil.AddFinalizer(ctx.Obj, common.CNDrainingFinalizer)
		return nil
	}); err != nil {
		return errors.WrapPrefix(err, "ensure finalizers for CNStore Pod", 0)
	}
	// remove draining start time in case we regret formal deletion decision
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	if err := ctx.Patch(pod, func() error {
		delete(pod.Annotations, v1alpha1.StoreDrainingStartAnno)
		delete(pod.Annotations, LockRestartSet)
		return nil
	}); err != nil {
		return errors.WrapPrefix(err, "removing CN draining start time", 0)
	}

	if _, ok := ctx.Obj.Labels[v1alpha1.DirectPodLabel]; ok {
		// GC idle direct-pod
		if ctx.Obj.Labels[v1alpha1.CNPodPhaseLabel] == v1alpha1.CNPodPhaseIdle || ctx.Obj.Labels[kruisev1alpha1.SpecifiedDeleteKey] != "" {
			if err := ctx.Patch(ctx.Obj, func() error {
				ctx.Obj.Labels[pub.LifecycleStateKey] = string(pub.LifecycleStatePreparingDelete)
				return nil
			}); err != nil {
				return errors.Wrap(err, 0)
			}
			return recon.ErrReSync("direct pod is idle, transfer to preparing delete")
		}
	}
	// policy based reconciliation
	if v1alpha1.IsPoolingPolicy(ctx.Obj) {
		return c.poolingCNReconcile(ctx)
	}
	return c.defaultCNNormalReconcile(ctx)
}

func (c *withCNSet) defaultCNNormalReconcile(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	uid := v1alpha1.GetCNPodUUID(pod)

	// sync CN labels for store and mark store as UP state
	var cnLabels []v1alpha1.CNLabel
	labelStr, ok := pod.Annotations[common.CNLabelAnnotation]
	if ok {
		err := json.Unmarshal([]byte(labelStr), &cnLabels)
		if err != nil {
			return errors.WrapPrefix(err, "unmarshal CNLabels", 0)
		}
	}

	var err error
	if c.cn.Spec.ScalingConfig.GetStoreDrainEnabled() {
		err = c.withMOClientSet(ctx, func(timeout context.Context, h *mocli.ClientSet) error {
			return h.Client.PatchCNStore(timeout, logpb.CNStateLabel{
				UUID:   uid,
				State:  metadata.WorkState_Working,
				Labels: common.ToStoreLabels(cnLabels),
			})
		})
	} else {
		err = c.withMOClientSet(ctx, func(timeout context.Context, h *mocli.ClientSet) error {
			return h.Client.UpdateCNLabel(timeout, logpb.CNStoreLabel{
				UUID:   uid,
				Labels: common.ToStoreLabels(cnLabels),
			})
		})
	}
	if err != nil {
		ctx.Log.Error(err, "update CN failed", "uuid", uid)
		return recon.ErrReSync("update cn failed", retryInterval)
	}
	ctx.Log.V(4).Info("successfully set CN working")

	return c.patchCNReadiness(ctx, corev1.ConditionTrue, messageCNStoreReady)
}

func (c *withCNSet) patchCNReadiness(ctx *recon.Context[*corev1.Pod], newC corev1.ConditionStatus, reason string) error {
	pod := ctx.Obj
	if err := ctx.PatchStatus(pod, func() error {
		cond := common.GetReadinessCondition(pod, common.CNStoreReadiness)
		if cond == nil {
			pod.Status.Conditions = append(pod.Status.Conditions, common.NewCNReadinessCondition(newC, reason))
		} else {
			if cond.Status != newC {
				cond.Status = newC
				cond.LastTransitionTime = metav1.Now()
			}
			cond.Message = reason
		}
		c.setCNState(pod, v1alpha1.CNStoreStateUp)
		return nil
	}); err != nil {
		return errors.WrapPrefix(err, "patch pod readiness", 0)
	}
	return nil
}

func (c *withCNSet) OnCordon(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	uid := v1alpha1.GetCNPodUUID(pod)
	ctx.Log.Info("call HAKeeper to cordon CN store", "uuid", uid)
	err := c.withMOClientSet(ctx, func(timeout context.Context, h *mocli.ClientSet) error {
		return h.Client.PatchCNStore(timeout, logpb.CNStateLabel{
			UUID:  uid,
			State: metadata.WorkState_Draining,
		})
	})
	if err != nil {
		return errors.WrapPrefix(err, "error cordon cn store", 0)
	}
	// set pod unready to unregister the pod from internal service
	return c.patchCNReadiness(ctx, corev1.ConditionFalse, messageCNCordon)
}

func (c *Controller) observe(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj

	// 1. process delete
	if pod.DeletionTimestamp != nil {
		return c.OnDeleted(ctx)
	}

	// 2. resolve CNSet
	cnSet, err := common.ResolveCNSet(ctx, pod)
	if err != nil {
		return errors.WrapPrefix(err, "error resolve CNSet", 0)
	}
	wc := &withCNSet{
		Controller: c,
		cn:         cnSet,
	}

	// 3. sync stats, including connections and deletion cost
	if err := wc.syncStats(ctx); err != nil {
		ctx.Log.Info("error sync stats", "error", err.Error())
		// sync stats should not block state sync, continue
	}

	// 4. optionally, store is asked to be cordoned
	if _, ok := pod.Annotations[v1alpha1.StoreCordonAnno]; ok {
		return wc.OnCordon(ctx)
	}

	lifecycleState := pod.Labels[pub.LifecycleStateKey]
	if lifecycleState == string(pub.LifecycleStatePreparingUpdate) {
		return wc.OnPreparingUpdate(ctx)
	} else if lifecycleState == string(pub.LifecycleStatePreparingDelete) {
		return wc.OnPreparingStop(ctx)
	}

	if err := wc.OnNormal(ctx); err != nil {
		return err
	}
	// trigger next reconciliation later to refresh the stats
	// TODO(aylei): better stats handling
	return recon.ErrReSync("resync", resyncInterval)
}

type connectionDiagnosis struct {
	Logger  logr.Logger
	Enabled bool
}

func (c *withCNSet) syncStats(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj

	startedTime := common.GetCNStartedTime(pod)
	if startedTime == nil {
		return errors.New("CN not started")
	}
	sc := &common.StoreScore{}
	previous, err := common.GetStoreScore(pod)
	if err == nil {
		sc = previous
	}
	// Invalidate the previous round before performing any query. A failed or partial
	// refresh must never leave an old zero looking like current evidence.
	sc.BeginObservation(startedTime)

	uid := v1alpha1.GetCNPodUUID(pod)
	moVersion := common.GetSemanticVersion(&pod.ObjectMeta)
	var queryAddress string
	if err := c.withMOClientSet(ctx, func(_ context.Context, handler *mocli.ClientSet) error {
		cn, ok := handler.StoreCache.GetCN(uid)
		if !ok {
			return gerrors.Errorf("CN with uuid %s not found", uid)
		}
		queryAddress = cn.QueryAddress
		return nil
	}); err != nil {
		ctx.Log.Info("error refresh stats, cn not found in store-cache", "error", err.Error())
		// BeginObservation has already invalidated this round. Persist that state so
		// IsSafeToReclaim remains the single reclaim-safety boundary, while preserving
		// the existing state-sync behavior when the CN is absent from the cache.
		return c.patchStoreStats(ctx, sc)
	}

	_, diagnosDraining := pod.Annotations[diagnosDrainingAnno]
	diagosis := &connectionDiagnosis{
		Logger:  ctx.Log,
		Enabled: diagnosDraining,
	}
	c.collectQueryStats(sc, queryAddress, moVersion, diagosis)

	return c.patchStoreStats(ctx, sc)
}

// collectQueryStats updates one observation round. Each observation is marked
// valid only after its query succeeds. A feature that is unavailable for the
// current MO version is explicitly treated as not required with a zero count.
func (c *Controller) collectQueryStats(sc *common.StoreScore, queryAddress string, moVersion semver.Version, diagosis *connectionDiagnosis) {
	count, err := c.getSessionCount(queryAddress, moVersion, diagosis)
	if err != nil {
		diagosis.Logger.Info("error get session count", "error", err.Error())
	} else {
		// update session count
		sc.SessionCount = count
		sc.SessionObserved = true
	}
	var pipelineCount int
	if v1alpha1.HasMOFeature(moVersion, v1alpha1.MOFeaturePipelineInfo) {
		pipelineCount, err = c.getPipelineCount(queryAddress, diagosis)
		if err != nil {
			diagosis.Logger.Info("error get pipeline count", "error", err.Error())
		} else {
			// update pipeline count
			sc.PipelineCount = pipelineCount
			sc.PipelineObserved = true
		}
	} else {
		// Pipeline observation is not required for versions without the API.
		sc.PipelineCount = 0
		sc.PipelineObserved = true
	}
	var replicaCount int
	if v1alpha1.HasMOFeature(moVersion, v1alpha1.MOFeatureShardingMigration) {
		replicaCount, err = c.getReplicaCount(queryAddress, diagosis)
		if err != nil {
			diagosis.Logger.Info("error get replica count", "error", err.Error())
		} else {
			sc.ReplicaCount = replicaCount
			sc.ReplicaObserved = true
		}
	} else {
		// Replica observation is not required for versions without sharding migration.
		sc.ReplicaCount = 0
		sc.ReplicaObserved = true
	}
}

func (c *Controller) patchStoreStats(ctx *recon.Context[*corev1.Pod], sc *common.StoreScore) error {
	pod := ctx.Obj
	err := ctx.Patch(pod, func() error {
		if err := common.SetStoreScore(pod, sc); err != nil {
			return err
		}
		pod.Annotations[common.DeletionCostAnno] = strconv.Itoa(sc.GenDeletionCost())
		if pod.Labels == nil {
			pod.Labels = map[string]string{}
		}
		pod.Labels[common.CNUUIDLabelKey] = v1alpha1.GetCNPodUUID(pod)
		// NB: store-connections anno is no longer used in mo-operator, but must be kept for external compatibility
		// ref:
		pod.Annotations[v1alpha1.StoreConnectionAnno] = strconv.Itoa(sc.PipelineCount + sc.SessionCount)
		return nil
	})
	if err != nil {
		return errors.WrapPrefix(err, "error patch stats to pod anno", 0)
	}
	return nil
}

func (c *Controller) getSessionCount(queryAddress string, moVersion semver.Version, diagosis *connectionDiagnosis) (int, error) {
	var count int
	resp, err := c.queryCli.ShowProcessList(context.Background(), queryAddress)
	if err != nil {
		return 0, errors.WrapPrefix(err, "show processlist", 0)
	}
	for _, sess := range resp.GetSessions() {
		if v1alpha1.HasMOFeature(moVersion, v1alpha1.MOFeatureSessionSource) {
			if sess.FromProxy {
				count++
			}
		} else {
			if sess.Account != "" && sess.Account != "sys" {
				count++
			}
		}
	}
	if diagosis.Enabled && count > 0 {
		diagosis.Logger.Info("CN sessions", "count", count, "detail", resp.GetSessions())
	}
	return count, nil
}

func (c *Controller) getReplicaCount(queryAddress string, diagosis *connectionDiagnosis) (int, error) {
	resp, err := c.queryCli.GetReplicaCount(context.Background(), queryAddress)
	if err != nil {
		return 0, errors.WrapPrefix(err, "get replica count", 0)
	}
	if diagosis.Enabled {
		diagosis.Logger.Info("CN replica count", "count", resp.GetCount())
	}
	return int(resp.GetCount()), nil
}

func (c *Controller) getPipelineCount(queryAddress string, diagosis *connectionDiagnosis) (int, error) {
	resp, err := c.queryCli.GetPipelineInfo(context.Background(), queryAddress)
	if err != nil {
		return 0, errors.WrapPrefix(err, "get pipeline info", 0)
	}
	if diagosis.Enabled {
		diagosis.Logger.Info("CN pipeline count", "count", resp.GetCount())
	}
	return int(resp.GetCount()), nil
}

func (c *withCNSet) withMOClientSet(ctx *recon.Context[*corev1.Pod], fn func(context.Context, *mocli.ClientSet) error) error {
	pod := ctx.Obj
	ls, err := common.ResolveLogSet(ctx, c.cn)
	if err != nil {
		return errors.WrapPrefix(err, "error resolve logset", 0)
	}
	if !recon.IsReady(ls) {
		return recon.ErrReSync(fmt.Sprintf("logset is not ready for Pod %s, cannot update CN labels", pod.Name), retryInterval)
	}
	handler, err := c.clientMgr.GetClient(ls)
	if err != nil {
		return errors.WrapPrefix(err, "get HAKeeper client", 0)
	}
	timeout, cancel := context.WithTimeout(context.Background(), mocli.DefaultRPCTimeout)
	defer cancel()
	if err := fn(timeout, handler); err != nil {
		return err
	}
	return nil
}

func (c *Controller) setCNState(pod *corev1.Pod, state string) {
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations[common.CNStateAnno] = state
}

func (c *Controller) Observe(ctx *recon.Context[*corev1.Pod]) (recon.Action[*corev1.Pod], error) {
	return nil, c.observe(ctx)
}

func (c *Controller) Finalize(ctx *recon.Context[*corev1.Pod]) (bool, error) {
	// deletion also handled by observe
	return true, c.observe(ctx)
}

func (c *Controller) Reconcile(mgr manager.Manager) error {
	c.apiReader = mgr.GetAPIReader()
	// Pod does not have generation field, so we cannot use the default reconcile
	return recon.Setup[*corev1.Pod](&corev1.Pod{}, "cnstore", mgr, c,
		recon.WithControllerOptions(controller.Options{
			MaxConcurrentReconciles: defaultConcurrency,
		}),
		recon.SkipStatusSync(),
		recon.WithPredicate(
			predicate.Or(predicate.LabelChangedPredicate{},
				predicate.GenerationChangedPredicate{},
				annotationChangedExcludeStats{},
				deletedPredicate{})),
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

// annotationChangedExcludeStats reconciles the object when annotations are changed (exclude stats)
type annotationChangedExcludeStats struct {
	predicate.Funcs
}

func (annotationChangedExcludeStats) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	oldAnnos := e.ObjectOld.GetAnnotations()
	newAnnos := e.ObjectNew.GetAnnotations()
	for k, v := range newAnnos {
		// exclude stats
		if k == common.DeletionCostAnno || k == v1alpha1.StoreConnectionAnno || k == v1alpha1.StoreScoreAnno {
			continue
		}
		// only consider newly added annotations or annotation value change, deletion of annotation key
		// do not need to be reconciled
		if oldAnnos[k] != v {
			return true
		}
	}
	return false
}

// deletePredicate reconciles the object when the deletionTimestamp field is changed
type deletedPredicate struct {
	predicate.Funcs
}

func (deletedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}

	return !reflect.DeepEqual(e.ObjectNew.GetDeletionTimestamp(), e.ObjectOld.GetDeletionTimestamp())
}
