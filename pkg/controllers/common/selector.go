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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ListPods(cli recon.KubeClient, opts ...client.ListOption) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := cli.List(podList, opts...); err != nil {
		return nil, err
	}
	return podList.Items, nil
}

func MustAsSelector(ps *metav1.LabelSelector) labels.Selector {
	ls, err := metav1.LabelSelectorAsSelector(ps)
	if err != nil {
		panic(errors.Wrap(err, "impossible path: LabelSelectorAsSelector failed"))
	}
	return ls
}

func MustNewRequirement(key string, op selection.Operator, vals []string, opts ...field.PathOption) labels.Requirement {
	r, err := labels.NewRequirement(key, op, vals)
	if err != nil {
		panic(errors.Wrap(err, "impossible path: new requirement failed"))
	}
	return *r
}

func MustEqual(key string, value string) labels.Requirement {
	return MustNewRequirement(key, selection.Equals, []string{value})
}
