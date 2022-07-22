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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func ChangedAfter(obj client.Object, mutateFn func()) bool {
	before := obj.DeepCopyObject().(client.Object)
	mutateFn()
	return reflect.DeepEqual(before, obj)
}
