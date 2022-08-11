// Copyright 2022 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"reflect"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	configmapMode = 0644
)

func Ignore(isErr func(error) bool, err error) error {
	if isErr(err) {
		return nil
	}
	return err
}

func WasDeleted(obj client.Object) bool {
	return obj.GetDeletionTimestamp() != nil
}

func IsFound(err error) (error, bool) {
	if err == nil {
		return nil, true
	}
	if apierrors.IsNotFound(err) {
		return nil, false
	}
	return err, false
}

func ChangedAfter(obj client.Object, mutateFn func() error) bool {
	before := obj.DeepCopyObject().(client.Object)
	mutateFn()
	return reflect.DeepEqual(before, obj)
}

type Predicate[E any] func(E) bool

func FindFirst[E any](list []E, predicate Predicate[E]) *E {
	for _, v := range list {
		if predicate(v) {
			return &v
		}
	}
	return nil
}

func WithVolumeName(name string) Predicate[corev1.Volume] {
	return func(v corev1.Volume) bool {
		return v.Name == name
	}
}

func ConfigMapVolume(name string) corev1.VolumeSource {
	mode := int32(configmapMode)
	return corev1.VolumeSource{
		ConfigMap: &corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: name},
			DefaultMode:          &mode,
		},
	}
}

func FieldRefEnv(key string, field string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: key,
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				APIVersion: "v1",
				FieldPath:  field,
			},
		},
	}
}

func Upsert[E comparable](list []E, elem E) []E {
	for _, o := range list {
		if o == elem {
			return list
		}
	}
	return append(list, elem)
}

func PodOrdinal(name string) (int, error) {
	ss := strings.Split(name, "-")
	suffix := ss[len(ss)-1]
	return strconv.Atoi(suffix)
}
