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

package cnset

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// CNStore refreshes Python status every 30 seconds. Allowing several
	// missed refreshes avoids treating a transient controller delay as a
	// worker replacement while still preventing an abandoned Ready annotation
	// from keeping a dead route open indefinitely.
	udfWorkerStatusMaxAge        = 2 * time.Minute
	udfWorkerStatusMaxFutureSkew = 30 * time.Second
)

// udfWorkerClientConfig is the single source for the typed Python client
// subtree written into the CN TOML. Keeping construction here lets status
// validation compare the persisted configuration with the policy that owns
// it, instead of treating any non-empty ConfigMap as ready.
func udfWorkerClientConfig(policy *v1alpha1.UDFWorkerPolicy) map[string]interface{} {
	if policy == nil || !policy.Enabled {
		return nil
	}
	client := policy.Client
	config := map[string]interface{}{
		"enabled":          true,
		"allow-unisolated": client.AllowUnisolated,
		"server-address":   fmt.Sprintf("127.0.0.1:%d", policy.EffectivePort()),
	}
	if client.MaxBatchBytes != 0 {
		config["max-batch-bytes"] = client.MaxBatchBytes
	}
	if client.MaxBatchRows != 0 {
		config["max-batch-rows"] = client.MaxBatchRows
	}
	if client.MaxInvocationRows != 0 {
		config["max-invocation-rows"] = client.MaxInvocationRows
	}
	if client.MaxInvocationResultBytes != 0 {
		config["max-invocation-result-bytes"] = client.MaxInvocationResultBytes
	}
	if client.MaxActiveInvocations != 0 {
		config["max-active-invocations"] = int64(client.MaxActiveInvocations)
	}
	if client.RequestTimeout != nil && client.RequestTimeout.Duration > 0 {
		config["request-timeout"] = client.RequestTimeout.Duration.String()
	}
	if client.MaxTerminalEntries != 0 {
		config["max-terminal-entries"] = client.MaxTerminalEntries
	}
	if client.MaxTerminalBytes != 0 {
		config["max-terminal-bytes"] = client.MaxTerminalBytes
	}
	if client.TerminalRecordTTL != nil && client.TerminalRecordTTL.Duration > 0 {
		config["terminal-record-ttl"] = client.TerminalRecordTTL.Duration.String()
	}
	return config
}

// hasExpectedUDFWorkerConfig verifies the exact ConfigMap entry selected by
// the current CN template. A stale entry, an arbitrary Python client subtree,
// or a malformed TOML document must leave the capability gate closed. The
// surrounding CN configuration is allowed to evolve independently; only the
// controller-owned typed Python subtree is compared here.
func hasExpectedUDFWorkerConfig(cm *corev1.ConfigMap, key string, policy *v1alpha1.UDFWorkerPolicy) bool {
	if cm == nil || policy == nil || !policy.Enabled || key == "" {
		return false
	}
	raw, ok := cm.Data[key]
	if !ok || raw == "" {
		return false
	}
	parsed := v1alpha1.NewTomlConfig(nil)
	if err := parsed.UnmarshalTOML([]byte(raw)); err != nil {
		return false
	}
	value := parsed.Get("cn", "python-udf-client")
	if value == nil {
		return false
	}
	section, err := value.AsToml()
	if err != nil {
		return false
	}
	return reflect.DeepEqual(normalizeUDFWorkerConfigValue(section.MP), normalizeUDFWorkerConfigValue(udfWorkerClientConfig(policy)))
}

func normalizeUDFWorkerConfigValue(value interface{}) interface{} {
	switch value := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(value))
		for key, nested := range value {
			result[key] = normalizeUDFWorkerConfigValue(nested)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(value))
		for i, nested := range value {
			result[i] = normalizeUDFWorkerConfigValue(nested)
		}
		return result
	case int:
		return int64(value)
	case int8:
		return int64(value)
	case int16:
		return int64(value)
	case int32:
		return int64(value)
	case int64:
		return value
	case uint:
		return uint64(value)
	case uint8:
		return uint64(value)
	case uint16:
		return uint64(value)
	case uint32:
		return uint64(value)
	case uint64:
		return value
	default:
		return value
	}
}

// validateUDFWorkerRendering is the controller-side mirror of the webhook's
// effective-policy checks. It runs before any CNSet-owned object is mutated so
// a restored object or a webhook bypass cannot leave a partial Worker configuration behind.
func validateUDFWorkerRendering(cn *v1alpha1.CNSet) error {
	if cn == nil {
		return nil
	}
	if err := cn.Spec.ValidateUDFWorkerConfiguration(); err != nil {
		return fmt.Errorf("invalid UDF worker policy: %w", err)
	}
	policy := cn.Spec.UDFWorker
	if policy == nil || !policy.Enabled || cn.Spec.Overlay == nil {
		return nil
	}
	if err := common.ValidateUDFWorkerPodOverlay(cn.Spec.Overlay); err != nil {
		return err
	}
	return nil
}

func retainUDFWorkerConditions(conditions []metav1.Condition, enabled bool) []metav1.Condition {
	allowed := map[string]struct{}{
		v1alpha1.UDFWorkerConditionEnabled:     {},
		v1alpha1.UDFWorkerConditionProvisioned: {},
	}
	if enabled {
		for _, typ := range []string{
			v1alpha1.UDFWorkerConditionDependencyReady,
			v1alpha1.UDFWorkerConditionCapabilityReady,
			v1alpha1.UDFWorkerConditionCapacityReady,
			v1alpha1.UDFWorkerConditionRouteReady,
			v1alpha1.UDFWorkerConditionClientConfigReady,
			v1alpha1.UDFWorkerConditionDraining,
			v1alpha1.UDFWorkerConditionDegraded,
		} {
			allowed[typ] = struct{}{}
		}
	}
	kept := conditions[:0]
	for _, condition := range conditions {
		if _, ok := allowed[condition.Type]; ok {
			kept = append(kept, condition)
		}
	}
	return kept
}

func syncUDFWorkerPodMarker(cn *v1alpha1.CNSet, meta *metav1.ObjectMeta) {
	if meta.Labels == nil {
		meta.Labels = map[string]string{}
	}
	if cn.Spec.UDFWorker != nil && cn.Spec.UDFWorker.Enabled {
		if meta.Annotations == nil {
			meta.Annotations = map[string]string{}
		}
		// Overlay.PodLabels runs before this function. Reassert the labels used
		// by the CNSet selector so an enabled worker cannot make its own Pod
		// disappear from the controller-owned Service.
		for key, value := range common.SubResourceLabels(cn) {
			if value != "" {
				meta.Labels[key] = value
			}
		}
		// MatrixOneCluster-generated CNSets carry the cluster selector on the
		// CNSet object. Project it here, after the user overlay, so the selector
		// remains available to cluster-level controllers without making the
		// controller-owned label part of the user-controlled overlay contract.
		if cluster := cn.Labels[common.MatrixoneClusterLabelKey]; cluster != "" {
			meta.Labels[common.MatrixoneClusterLabelKey] = cluster
		}
		meta.Labels[v1alpha1.UDFWorkerEnabledLabel] = v1alpha1.UDFWorkerEnabledValue
		meta.Annotations[v1alpha1.UDFWorkerGenerationAnno] = v1alpha1.UDFWorkerPolicyGeneration(cn.Spec.UDFWorker)
	} else {
		delete(meta.Labels, v1alpha1.UDFWorkerEnabledLabel)
		delete(meta.Annotations, v1alpha1.UDFWorkerGenerationAnno)
	}
}

func syncUDFWorkerStatus(cn *v1alpha1.CNSet, cs *kruisev1alpha1.CloneSet, configReady bool) {
	syncUDFWorkerStatusWithPods(cn, cs, nil, configReady)
}

func syncUDFWorkerStatusWithPods(cn *v1alpha1.CNSet, cs *kruisev1alpha1.CloneSet, pods []corev1.Pod, configReady bool) {
	policy := cn.Spec.UDFWorker
	status := &cn.Status.UDFWorker
	status.Topology = policy.EffectiveTopology()
	status.DesiredWorkers = 0
	status.ReadyWorkers = 0
	status.Generation = ""

	if policy == nil || !policy.Enabled {
		status.Conditions = retainUDFWorkerConditions(status.Conditions, false)
		setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionEnabled, metav1.ConditionFalse,
			"Disabled", "Python UDF worker is disabled")
		setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionProvisioned, metav1.ConditionFalse,
			"Disabled", "no Python UDF worker is rendered")
		return
	}

	status.Conditions = retainUDFWorkerConditions(status.Conditions, true)
	// The ConfigMap digest alone does not identify the Worker workload: an
	// image, resource limit, port, launcher, or client bound can change while
	// the surrounding CN TOML stays equivalent. Publish a stable digest of the
	// complete accepted policy so route/status consumers cannot treat such a
	// replacement as the same Worker generation.
	status.Generation = v1alpha1.UDFWorkerPolicyGeneration(policy)
	status.DesiredWorkers = cn.Spec.Replicas
	provisioned := false
	if cs != nil {
		// The current same-Pod implementation has one worker container per CN
		// Pod. CloneSet readiness therefore gives a bounded aggregate count
		// without inventing a separate WorkerSet or endpoint registry.
		status.ReadyWorkers = cs.Status.ReadyReplicas
		provisioned = hasAuthoritativeUDFWorker(policy, cs)
	}
	if !provisioned {
		status.ReadyWorkers = 0
	}
	setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionEnabled, metav1.ConditionTrue,
		"PolicyAccepted", "current paired same-Pod policy is accepted")
	if provisioned {
		setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionProvisioned, metav1.ConditionTrue,
			"PodTemplateRendered", "the controller owns the UDF worker container")
	} else {
		setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionProvisioned, metav1.ConditionFalse,
			"WorkerTemplateNotAuthoritative", "the CN Pod template does not contain the current worker command, image, port, and loopback bind")
	}
	setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionDependencyReady, metav1.ConditionTrue,
		"DirectImage", "python-image has no HAKeeper/FileService launcher dependency")
	setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionCapacityReady, metav1.ConditionTrue,
		"PolicyValidated", "worker limits and CN client bounds passed admission")
	if configReady {
		setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionClientConfigReady, metav1.ConditionTrue,
			"ConfigRendered", "CN client configuration exists and the CNSet template references it")
	} else {
		setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionClientConfigReady, metav1.ConditionFalse,
			"ConfigNotReady", "the CN client ConfigMap is not yet referenced by the CN Pod template")
	}
	capabilityReady, capabilityReadyWorkers, capabilityReason, capabilityMessage := aggregateUDFWorkerCapability(policy, status.Generation, status.DesiredWorkers, pods)
	if pods == nil {
		capabilityReady = false
		capabilityReason = "RuntimeStatusBridgeUnavailable"
		capabilityMessage = "the MO CN query status bridge has not reported a per-Pod capability handshake"
	} else {
		status.ReadyWorkers = capabilityReadyWorkers
	}
	if capabilityReady {
		setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionCapabilityReady, metav1.ConditionTrue,
			"CapabilityHandshakeReady", capabilityMessage)
	} else {
		setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionCapabilityReady, metav1.ConditionFalse,
			capabilityReason, capabilityMessage)
	}
	routeReady := capabilityReady && configReady && provisioned
	if routeReady {
		setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionRouteReady, metav1.ConditionTrue,
			"CapabilityHandshakeReady", "all current CN-local Python capability handshakes are ready")
		setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionDegraded, metav1.ConditionFalse,
			"Ready", "ordinary CN readiness and the Python capability route are ready")
	} else {
		setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionRouteReady, metav1.ConditionFalse,
			"CapabilityNotReady", "Python route remains closed until configuration and every CN-local Gateway handshake are observed")
		setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionDegraded, metav1.ConditionTrue,
			capabilityReason, "ordinary CN readiness is independent of the Python capability condition")
	}
	setUDFWorkerCondition(cn, status, v1alpha1.UDFWorkerConditionDraining, metav1.ConditionFalse,
		"NoDrainRequested", "the CNSet is not draining its Python UDF worker")
}

func aggregateUDFWorkerCapability(policy *v1alpha1.UDFWorkerPolicy, generation string, desiredWorkers int32, pods []corev1.Pod) (bool, int32, string, string) {
	if policy == nil || !policy.Enabled {
		return false, 0, "Disabled", "Python UDF worker is disabled"
	}
	if desiredWorkers <= 0 {
		return false, 0, "NoWorkersDesired", "the CNSet does not currently desire a Python UDF worker Pod"
	}
	if len(pods) == 0 {
		return false, 0, "NoWorkerPods", "no CN Pod has reported a Python capability handshake"
	}
	ready := 0
	reason := "RuntimeStatusBridgeUnavailable"
	message := "one or more CN-local Python capability handshakes are missing"
	now := time.Now()
	for i := range pods {
		pod := &pods[i]
		if pod.Labels[v1alpha1.UDFWorkerEnabledLabel] != v1alpha1.UDFWorkerEnabledValue {
			continue
		}
		var observation v1alpha1.UDFWorkerPodStatus
		if raw := pod.Annotations[v1alpha1.UDFWorkerStatusAnno]; raw != "" {
			if len(raw) > v1alpha1.UDFWorkerStatusMaxAnnotationBytes {
				reason = v1alpha1.UDFWorkerStatusErrorInvalid
				message = "a CN Pod published an oversized Python capability status"
				continue
			}
			if err := json.Unmarshal([]byte(raw), &observation); err != nil || v1alpha1.ValidateUDFWorkerPodStatus(&observation) != nil {
				reason = v1alpha1.UDFWorkerStatusErrorInvalid
				message = "a CN Pod published an invalid Python capability status"
				continue
			}
		}
		if observation.ObservedAt.Time.IsZero() {
			if pod.Annotations[v1alpha1.UDFWorkerStatusAnno] != "" {
				reason = v1alpha1.UDFWorkerStatusErrorStale
				message = "a CN Pod published Python capability status without an observation time"
			}
			continue
		}
		if observation.ObservedAt.Time.Before(now.Add(-udfWorkerStatusMaxAge)) ||
			observation.ObservedAt.Time.After(now.Add(udfWorkerStatusMaxFutureSkew)) {
			reason = v1alpha1.UDFWorkerStatusErrorStale
			message = "a CN Pod published Python capability status outside the freshness window"
			continue
		}
		expectedCNUUID := v1alpha1.GetCNPodUUID(pod)
		if observation.PodUID != string(pod.UID) || observation.CNUUID != expectedCNUUID || observation.Generation != generation {
			reason = v1alpha1.UDFWorkerStatusErrorIdentityMismatch
			message = v1alpha1.UDFWorkerStatusReasonIdentityMismatch
			continue
		}

		if observation.Ready && observation.PodUID == string(pod.UID) &&
			observation.CNUUID == expectedCNUUID && observation.Generation == generation &&
			observation.LeaseEpoch != 0 && observation.ErrorClass == "" {
			ready++
			continue
		}
		if observation.ErrorClass != "" {
			reason = observation.ErrorClass
			message = observation.Reason
		}
	}
	if ready == int(desiredWorkers) {
		return true, int32(ready), "CapabilityHandshakeReady", fmt.Sprintf("%d CN-local Python capability handshakes are ready", ready)
	}
	return false, int32(ready), reason, message
}

func hasAuthoritativeUDFWorker(policy *v1alpha1.UDFWorkerPolicy, cs *kruisev1alpha1.CloneSet) bool {
	if policy == nil || !policy.Enabled || cs == nil {
		return false
	}
	podSpec := cs.Spec.Template.Spec
	if podSpec.HostNetwork || podSpec.HostPID ||
		(podSpec.ShareProcessNamespace != nil && *podSpec.ShareProcessNamespace) ||
		podSpec.InitContainers != nil ||
		(podSpec.SecurityContext != nil && !equality.Semantic.DeepEqual(podSpec.SecurityContext, &corev1.PodSecurityContext{})) ||
		podSpec.ServiceAccountName != "" || podSpec.RuntimeClassName != nil {
		return false
	}
	// The current implementation is a same-Pod pair rendered by this
	// controller. An extra or reordered container would make the ownership and
	// process-boundary contract ambiguous, even if a correctly named worker is
	// present.
	if len(cs.Spec.Template.Spec.Containers) != 2 ||
		cs.Spec.Template.Spec.Containers[0].Name != v1alpha1.ContainerMain {
		return false
	}
	var worker *corev1.Container
	for i := range cs.Spec.Template.Spec.Containers {
		container := &cs.Spec.Template.Spec.Containers[i]
		if container.Name != v1alpha1.ContainerUDFWorker {
			continue
		}
		if worker != nil {
			return false
		}
		worker = container
	}
	if worker == nil {
		return false
	}
	for _, container := range podSpec.Containers {
		for _, port := range container.Ports {
			if port.HostPort != 0 {
				return false
			}
		}
	}
	expected := authoritativeUDFWorkerContainer(policy)
	return equality.Semantic.DeepEqual(*worker, expected)
}

func authoritativeUDFWorkerContainer(policy *v1alpha1.UDFWorkerPolicy) corev1.Container {
	port := policy.EffectivePort()
	runAsUser := v1alpha1.UDFWorkerRunAsUser
	runAsGroup := v1alpha1.UDFWorkerRunAsGroup
	return corev1.Container{
		Name:                     v1alpha1.ContainerUDFWorker,
		Image:                    policy.Worker.Image,
		ImagePullPolicy:          policy.EffectiveImagePullPolicy(),
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		Command:                  []string{"/usr/bin/tini", "--", "python", "-u", "worker.py"},
		Args:                     []string{fmt.Sprintf("--address=grpc://127.0.0.1:%d", port)},
		Resources:                *policy.Worker.Resources.DeepCopy(),
		Ports: []corev1.ContainerPort{{
			Name:          "flight",
			ContainerPort: port,
			Protocol:      corev1.ProtocolTCP,
		}},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: func() *bool { v := false; return &v }(),
			RunAsNonRoot:             func() *bool { v := true; return &v }(),
			RunAsUser:                &runAsUser,
			RunAsGroup:               &runAsGroup,
		},
	}
}

func setUDFWorkerCondition(cn *v1alpha1.CNSet, status *v1alpha1.UDFWorkerStatus, typ string, state metav1.ConditionStatus, reason, message string) {
	for i := range status.Conditions {
		if status.Conditions[i].Type != typ {
			continue
		}
		if status.Conditions[i].Status == state && status.Conditions[i].Reason == reason && status.Conditions[i].Message == message &&
			status.Conditions[i].ObservedGeneration == cn.Generation {
			return
		}
		status.Conditions[i] = metav1.Condition{
			Type:               typ,
			Status:             state,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: cn.Generation,
			LastTransitionTime: metav1.Now(),
		}
		return
	}
	status.Conditions = append(status.Conditions, metav1.Condition{
		Type:               typ,
		Status:             state,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: cn.Generation,
		LastTransitionTime: metav1.Now(),
	})
}
