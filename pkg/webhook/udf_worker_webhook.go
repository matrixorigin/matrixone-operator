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

package webhook

import (
	"time"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// defaultUDFWorkerPolicy applies only representation defaults. It never turns
// the feature on and never invents topology or launcher choices.
func defaultUDFWorkerPolicy(policy *v1alpha1.UDFWorkerPolicy) {
	if policy == nil || !policy.Enabled {
		return
	}
	if policy.Worker.Port == 0 {
		policy.Worker.Port = v1alpha1.ContainerUDFWorkerDefaultPort
	}
	if policy.Worker.ImagePullPolicy == "" {
		policy.Worker.ImagePullPolicy = corev1.PullIfNotPresent
	}
}

func validateUDFWorkerPolicy(spec *v1alpha1.CNSetSpec, path *field.Path) field.ErrorList {
	var errs field.ErrorList

	if spec.PythonUdfSidecar != nil {
		errs = append(errs, field.Forbidden(path.Child("pythonUdfSidecar"),
			"LegacyPythonUdfSidecarUnsupported: recreate the function with spec.udfWorker"))
	}

	// Python client settings are generated from the typed policy. Leaving this
	// arbitrary TOML escape hatch enabled would let a stale/older controller
	// turn Python on without passing the current admission contract.
	if spec.Config != nil && spec.Config.Get("cn", "python-udf-client") != nil {
		errs = append(errs, field.Forbidden(path.Child("config"),
			"PythonClientConfigManagedByUDFWorkerPolicy"))
	}

	policy := spec.UDFWorker
	if policy == nil {
		return errs
	}

	workerPath := path.Child("udfWorker")
	errs = append(errs, validateUDFClientConfigFields(&policy.Client, workerPath)...)
	if !policy.Enabled {
		if policy != nil && policy.Topology != "" && policy.Topology != v1alpha1.UDFWorkerTopologyDisabled &&
			policy.Topology != v1alpha1.UDFWorkerTopologyPaired && policy.Topology != v1alpha1.UDFWorkerTopologyPool {
			errs = append(errs, field.Invalid(path.Child("udfWorker", "topology"), policy.Topology,
				"UnsupportedUDFWorkerTopology"))
		}
		return errs
	}

	switch policy.Topology {
	case "":
		errs = append(errs, field.Required(workerPath.Child("topology"), "UDFWorkerTopologyRequired"))
	case v1alpha1.UDFWorkerTopologyPaired:
		// The first real Operator implementation is CN-local same-Pod.
	case v1alpha1.UDFWorkerTopologyPool:
		errs = append(errs, field.Forbidden(workerPath.Child("topology"),
			"UnsupportedUDFWorkerTopology: pool requires a lease-aware external scheduler"))
	default:
		errs = append(errs, field.Invalid(workerPath.Child("topology"), policy.Topology,
			"UnsupportedUDFWorkerTopology"))
	}

	switch policy.Launcher {
	case "":
		errs = append(errs, field.Required(workerPath.Child("launcher"), "UDFWorkerLauncherRequired"))
	case v1alpha1.UDFWorkerLauncherMOService:
		errs = append(errs, field.Forbidden(workerPath.Child("launcher"),
			"UnsupportedSamePodMoServiceLauncher"))
	case v1alpha1.UDFWorkerLauncherPythonImage:
	default:
		errs = append(errs, field.Invalid(workerPath.Child("launcher"), policy.Launcher,
			"UnsupportedUDFWorkerLauncher"))
	}

	if !v1alpha1.IsImmutableImageDigest(policy.Worker.Image) {
		errs = append(errs, field.Invalid(workerPath.Child("image"), policy.Worker.Image,
			"UDFWorkerImageMustUseDigest"))
	}
	if policy.Worker.Port != 0 && (policy.Worker.Port < 1 || policy.Worker.Port > 65535) {
		errs = append(errs, field.Invalid(workerPath.Child("port"), policy.Worker.Port,
			"UDFWorkerPortOutOfRange"))
	}
	if v1alpha1.IsCNPodPortReservedForUDFWorker(policy.EffectivePort()) {
		errs = append(errs, field.Invalid(workerPath.Child("port"), policy.EffectivePort(),
			"UDFWorkerPortConflictsWithCN"))
	}
	if policy.Worker.ImagePullPolicy != "" && policy.Worker.ImagePullPolicy != "Always" &&
		policy.Worker.ImagePullPolicy != "Never" && policy.Worker.ImagePullPolicy != "IfNotPresent" {
		errs = append(errs, field.Invalid(workerPath.Child("imagePullPolicy"), policy.Worker.ImagePullPolicy,
			"UnsupportedUDFWorkerImagePullPolicy"))
	}

	if !v1alpha1.IsFinitePositiveQuantity(policy.Worker.Resources.Limits.Cpu()) {
		errs = append(errs, field.Required(workerPath.Child("resources", "limits", "cpu"),
			"UDFWorkerResourceLimitsRequired"))
	}
	if !v1alpha1.IsFinitePositiveQuantity(policy.Worker.Resources.Limits.Memory()) {
		errs = append(errs, field.Required(workerPath.Child("resources", "limits", "memory"),
			"UDFWorkerResourceLimitsRequired"))
	}
	cpuLimit := policy.Worker.Resources.Limits.Cpu()
	memoryLimit := policy.Worker.Resources.Limits.Memory()
	limitsBelowRequests := cpuLimit != nil && memoryLimit != nil &&
		(policy.Worker.Resources.Requests.Cpu().Cmp(*cpuLimit) > 0 ||
			policy.Worker.Resources.Requests.Memory().Cmp(*memoryLimit) > 0)
	if limitsBelowRequests {
		errs = append(errs, field.Invalid(workerPath.Child("resources"), policy.Worker.Resources,
			"UDFWorkerLimitBelowRequest"))
	}
	if !v1alpha1.IsFiniteNonNegativeQuantity(policy.Worker.Resources.Requests.Cpu()) ||
		!v1alpha1.IsFiniteNonNegativeQuantity(policy.Worker.Resources.Requests.Memory()) {
		errs = append(errs, field.Invalid(workerPath.Child("resources", "requests"), policy.Worker.Resources.Requests,
			"UDFWorkerRequestsOutOfRange"))
	}
	if len(policy.Worker.NodeSelector) != 0 || len(policy.Worker.Tolerations) != 0 || policy.Worker.ServiceAccountName != "" {
		errs = append(errs, field.Forbidden(workerPath,
			"UnsupportedSamePodWorkerScheduling"))
	}
	if policy.Pool != nil {
		errs = append(errs, field.Forbidden(path.Child("udfWorker", "pool"),
			"UDFWorkerPoolConfigNotAllowedForPaired"))
	}
	if policy.Worker.UpdateStrategy.DrainTimeout != nil {
		errs = append(errs, field.Forbidden(workerPath.Child("updateStrategy"),
			"UnsupportedSamePodWorkerUpdateStrategy"))
	}
	if policy.Worker.MOService != nil {
		errs = append(errs, field.Forbidden(workerPath.Child("moService"),
			"UnsupportedSamePodMoServiceConfig"))
	}
	if !policy.Client.AllowUnisolated {
		errs = append(errs, field.Forbidden(workerPath.Child("client", "allowUnisolated"),
			"UDFWorkerRequiresAllowUnisolated"))
	}

	errs = append(errs, validateUDFWorkerPodOverlay(spec, path, policy.Enabled)...)

	return errs
}

func validateUDFClientConfigFields(client *v1alpha1.UDFClientConfig, workerPath *field.Path) field.ErrorList {
	var errs field.ErrorList
	validateNonNegative := func(value int64, child, reason string, max int64) {
		if value < 0 || value > max {
			errs = append(errs, field.Invalid(workerPath.Child("client", child), value, reason))
		}
	}
	validateNonNegative(client.MaxBatchBytes, "maxBatchBytes", "UDFClientMaxBatchBytesOutOfRange", 1<<30)
	validateNonNegative(client.MaxBatchRows, "maxBatchRows", "UDFClientMaxBatchRowsOutOfRange", 1<<30)
	validateNonNegative(client.MaxInvocationRows, "maxInvocationRows", "UDFClientMaxInvocationRowsOutOfRange", 1<<32)
	validateNonNegative(client.MaxInvocationResultBytes, "maxInvocationResultBytes", "UDFClientMaxInvocationResultBytesOutOfRange", 1<<40)
	if client.MaxActiveInvocations < 0 || client.MaxActiveInvocations > 1<<20 {
		errs = append(errs, field.Invalid(workerPath.Child("client", "maxActiveInvocations"), client.MaxActiveInvocations,
			"UDFClientMaxActiveInvocationsOutOfRange"))
	}
	if client.RequestTimeout != nil && (client.RequestTimeout.Duration < 0 || client.RequestTimeout.Duration > time.Hour) {
		errs = append(errs, field.Invalid(workerPath.Child("client", "requestTimeout"), client.RequestTimeout,
			"UDFClientRequestTimeoutOutOfRange"))
	}
	validateNonNegative(client.MaxTerminalEntries, "maxTerminalEntries", "UDFClientMaxTerminalEntriesOutOfRange", 1<<30)
	validateNonNegative(client.MaxTerminalBytes, "maxTerminalBytes", "UDFClientMaxTerminalBytesOutOfRange", 1<<40)
	if client.TerminalRecordTTL != nil && client.TerminalRecordTTL.Duration < 0 {
		errs = append(errs, field.Invalid(workerPath.Child("client", "terminalRecordTTL"), client.TerminalRecordTTL,
			"UDFClientTerminalRecordTTLOutOfRange"))
	}
	return errs
}

// validateUDFWorkerPodOverlay validates the Pod fields that become dangerous
// only after the effective policy enables a same-Pod worker. MatrixOneCluster
// applies its cluster policy to generated CN groups in the controller, so its
// webhook also calls this helper against that effective policy before the
// generated CNSet exists.
func validateUDFWorkerPodOverlay(spec *v1alpha1.CNSetSpec, path *field.Path, enabled bool) field.ErrorList {
	if !enabled || spec.Overlay == nil {
		return nil
	}

	var errs field.ErrorList
	overlay := spec.Overlay
	{
		for key := range overlay.PodLabels {
			if common.IsUDFWorkerControllerOwnedPodLabel(key) {
				errs = append(errs, field.Forbidden(path.Child("overlay", "podLabels").Key(key),
					"PythonEnabledCNSetDisallowsControllerOwnedPodLabel"))
			}
		}
		for key := range overlay.PodAnnotations {
			if common.IsUDFWorkerControllerOwnedPodAnnotation(key) {
				errs = append(errs, field.Forbidden(path.Child("overlay", "podAnnotations").Key(key),
					"PythonEnabledCNSetDisallowsControllerOwnedPodAnnotation"))
			}
		}
		if len(overlay.SidecarContainers) > 0 {
			errs = append(errs, field.Forbidden(path.Child("overlay", "sidecarContainers"),
				"PythonEnabledCNSetDisallowsOverlaySidecars"))
		}
		if overlay.ShareProcessNamespace != nil && *overlay.ShareProcessNamespace {
			errs = append(errs, field.Forbidden(path.Child("overlay", "shareProcessNamespace"),
				"PythonEnabledCNSetDisallowsSharedProcessNamespace"))
		}
		if len(overlay.Volumes) > 0 {
			errs = append(errs, field.Forbidden(path.Child("overlay", "volumes"),
				"PythonEnabledCNSetDisallowsOverlayVolumes"))
		}
		if overlay.InitContainers != nil {
			errs = append(errs, field.Forbidden(path.Child("overlay", "initContainers"),
				"PythonEnabledCNSetDisallowsOverlayInitContainers"))
		}
		if overlay.SecurityContext != nil {
			errs = append(errs, field.Forbidden(path.Child("overlay", "securityContext"),
				"PythonEnabledCNSetDisallowsOverlaySecurityContext"))
		}
		if overlay.ServiceAccountName != "" {
			errs = append(errs, field.Forbidden(path.Child("overlay", "serviceAccountName"),
				"PythonEnabledCNSetDisallowsOverlayServiceAccount"))
		}
		if overlay.RuntimeClassName != nil {
			errs = append(errs, field.Forbidden(path.Child("overlay", "runtimeClassName"),
				"PythonEnabledCNSetDisallowsUnapprovedRuntimeClass"))
		}
	}

	return errs
}
