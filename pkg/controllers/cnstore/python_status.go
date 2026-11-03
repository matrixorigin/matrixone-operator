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
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-errors/errors"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/querycli"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const pythonLanguage = "python"

func (c *withCNSet) queryPythonUDFStatus(ctx context.Context, pod *corev1.Pod, address string) v1alpha1.UDFWorkerPodStatus {
	status := c.unavailablePythonUDFStatus(pod)
	if c == nil || pod == nil || c.cn == nil || !c.cn.Spec.UDFWorker.IsEnabled() ||
		c.Controller == nil || c.Controller.queryCli == nil {
		return status
	}
	if ctx == nil {
		ctx = context.Background()
	}
	observed, err := c.queryCli.GetPythonUdfStatus(ctx, address)
	if err != nil {
		return status
	}
	if err := validatePythonUDFStatusBounds(observed); err != nil {
		status.ErrorClass = v1alpha1.UDFWorkerStatusErrorInvalid
		status.Reason = v1alpha1.UDFWorkerStatusReasonInvalid
		return status
	}
	status.CNUUID = observed.CNUUID
	status.Ready = observed.Ready
	status.ErrorClass = observed.ErrorClass
	status.Reason = observed.Reason
	status.ProtocolVersion = observed.ProtocolVersion
	status.ABIContract = observed.ABIContract
	status.AdapterVersion = observed.AdapterVersion
	status.SDKVersion = observed.SDKVersion
	status.DefinitionSchemaVersion = observed.DefinitionSchemaVersion
	status.PlanContractVersion = observed.PlanContractVersion
	status.TypeDescriptorContract = observed.TypeDescriptorContract
	status.TimezoneDatabaseVersion = observed.TimezoneDatabaseVersion
	status.WindowBatches = observed.WindowBatches
	status.CumulativeAck = observed.CumulativeAck
	status.MaxExecutionFrameBytes = observed.MaxExecutionFrameBytes
	status.MaxHandlerProcesses = observed.MaxHandlerProcesses
	status.MaxAccountHandlerProcesses = observed.MaxAccountHandlerProcesses
	status.MaxOwnerHandlerProcesses = observed.MaxOwnerHandlerProcesses
	status.LeaseEpoch = observed.LeaseEpoch
	status.Modes = append([]string(nil), observed.Modes...)
	status.NullPolicies = append([]string(nil), observed.NullPolicies...)
	status.Generation = pod.Annotations[v1alpha1.UDFWorkerGenerationAnno]
	status.ObservedAt = metav1.Now()

	expectedUID := v1alpha1.GetCNPodUUID(pod)
	expectedGeneration := v1alpha1.UDFWorkerPolicyGeneration(c.cn.Spec.UDFWorker)
	if observed.CNUUID != expectedUID || status.Generation != expectedGeneration || observed.Language != pythonLanguage ||
		!observed.Enabled || !observed.AllowUnisolated ||
		(observed.Ready && observed.LeaseEpoch == 0) ||
		(observed.Ready && observed.ErrorClass != "") {
		status.Ready = false
		status.ErrorClass = v1alpha1.UDFWorkerStatusErrorInvalid
		status.Reason = v1alpha1.UDFWorkerStatusReasonInvalid
		if observed.CNUUID != expectedUID || status.Generation != expectedGeneration {
			status.ErrorClass = v1alpha1.UDFWorkerStatusErrorIdentityMismatch
			status.Reason = v1alpha1.UDFWorkerStatusReasonIdentityMismatch
		} else if !observed.Enabled || !observed.AllowUnisolated {
			status.ErrorClass = v1alpha1.UDFWorkerStatusErrorPolicyMismatch
			status.Reason = v1alpha1.UDFWorkerStatusReasonPolicyMismatch
		}
	}
	return status
}

func validatePythonUDFStatusBounds(status *querycli.PythonUDFStatus) error {
	if status == nil {
		return fmt.Errorf("nil Python UDF status")
	}
	for name, value := range map[string]string{
		"cnUUID":                  status.CNUUID,
		"language":                status.Language,
		"errorClass":              status.ErrorClass,
		"reason":                  status.Reason,
		"abiContract":             status.ABIContract,
		"adapterVersion":          status.AdapterVersion,
		"sdkVersion":              status.SDKVersion,
		"typeDescriptorContract":  status.TypeDescriptorContract,
		"timezoneDatabaseVersion": status.TimezoneDatabaseVersion,
	} {
		if len(value) > v1alpha1.UDFWorkerStatusMaxStringBytes {
			return fmt.Errorf("Python UDF status %s exceeds %d bytes", name, v1alpha1.UDFWorkerStatusMaxStringBytes)
		}
	}
	if !v1alpha1.IsValidUDFRuntimeErrorClass(status.ErrorClass) {
		return fmt.Errorf("Python UDF status errorClass is not a recognized condition reason")
	}
	for name, values := range map[string][]string{
		"modes":        status.Modes,
		"nullPolicies": status.NullPolicies,
	} {
		if len(values) > v1alpha1.UDFWorkerStatusMaxListItems {
			return fmt.Errorf("Python UDF status %s has too many entries", name)
		}
		for _, value := range values {
			if len(value) > v1alpha1.UDFWorkerStatusMaxListItemBytes {
				return fmt.Errorf("Python UDF status %s entry exceeds %d bytes", name, v1alpha1.UDFWorkerStatusMaxListItemBytes)
			}
		}
	}
	return nil
}

func (c *withCNSet) unavailablePythonUDFStatus(pod *corev1.Pod) v1alpha1.UDFWorkerPodStatus {
	status := v1alpha1.UDFWorkerPodStatus{
		Ready:      false,
		ErrorClass: v1alpha1.UDFWorkerStatusErrorQueryUnavailable,
		Reason:     v1alpha1.UDFWorkerStatusReasonQueryUnavailable,
		ObservedAt: metav1.Now(),
	}
	if pod == nil {
		status.ErrorClass = v1alpha1.UDFWorkerStatusErrorInvalid
		status.Reason = v1alpha1.UDFWorkerStatusReasonInvalid
		return status
	}
	status.PodUID = string(pod.UID)
	status.CNUUID = v1alpha1.GetCNPodUUID(pod)
	if c != nil && c.cn != nil && c.cn.Spec.UDFWorker.IsEnabled() {
		status.Generation = pod.Annotations[v1alpha1.UDFWorkerGenerationAnno]
	}
	if c == nil {
		return status
	}
	if c.cn == nil || !c.cn.Spec.UDFWorker.IsEnabled() {
		status.ErrorClass = ""
		status.Reason = "Disabled"
	}
	return status
}

func (c *withCNSet) patchPythonUDFStatus(ctx *recon.Context[*corev1.Pod], status v1alpha1.UDFWorkerPodStatus) error {
	if c.cn == nil || !c.cn.Spec.UDFWorker.IsEnabled() {
		if _, ok := ctx.Obj.Annotations[v1alpha1.UDFWorkerStatusAnno]; !ok {
			return nil
		}
		return ctx.Patch(ctx.Obj, func() error {
			delete(ctx.Obj.Annotations, v1alpha1.UDFWorkerStatusAnno)
			return nil
		})
	}
	payload, err := json.Marshal(status)
	if err != nil {
		return errors.WrapPrefix(err, "marshal Python UDF status", 0)
	}
	if len(payload) > v1alpha1.UDFWorkerStatusMaxAnnotationBytes {
		// Replace the payload instead of leaving an older Ready observation in
		// place. The fallback contains only the current Pod fence and a stable
		// failure class, so a future status extension cannot turn annotation
		// size into a stale-capability bug.
		status = v1alpha1.UDFWorkerPodStatus{
			PodUID:     status.PodUID,
			Generation: status.Generation,
			Ready:      false,
			ErrorClass: v1alpha1.UDFWorkerStatusErrorInvalid,
			Reason:     v1alpha1.UDFWorkerStatusReasonInvalid,
			ObservedAt: metav1.Now(),
		}
		payload, err = json.Marshal(status)
		if err != nil {
			return errors.WrapPrefix(err, "marshal bounded Python UDF status", 0)
		}
	}
	return ctx.Patch(ctx.Obj, func() error {
		if ctx.Obj.Annotations == nil {
			ctx.Obj.Annotations = map[string]string{}
		}
		ctx.Obj.Annotations[v1alpha1.UDFWorkerStatusAnno] = string(payload)
		return nil
	})
}
