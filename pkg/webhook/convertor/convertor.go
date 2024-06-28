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

package convertor

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubernetes/pkg/apis/core"
	apiscorev1 "k8s.io/kubernetes/pkg/apis/core/v1"
)

func ConvertResourceRequirements(rr *corev1.ResourceRequirements) (*core.ResourceRequirements, error) {
	out := &core.ResourceRequirements{}
	err := apiscorev1.Convert_v1_ResourceRequirements_To_core_ResourceRequirements(rr, out, nil)
	return out, err
}

func ConvertPodTemplate(pt *corev1.PodTemplate) (*core.PodTemplate, error) {
	out := &core.PodTemplate{}
	err := apiscorev1.Convert_v1_PodTemplate_To_core_PodTemplate(pt, out, nil)
	return out, err
}

func ConvertTolerations(tolerations []corev1.Toleration) ([]core.Toleration, error) {
	outSlice := make([]core.Toleration, 0, len(tolerations))
	var errs []error
	for _, toleration := range tolerations {
		in, out := toleration, core.Toleration{}
		err := apiscorev1.Convert_v1_Toleration_To_core_Toleration(&in, &out, nil)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		outSlice = append(outSlice, out)
	}

	return outSlice, errors.NewAggregate(errs)
}
