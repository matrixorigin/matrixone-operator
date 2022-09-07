// Copyright 2022 Matrix Origin
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
package reconciler

import (
	"fmt"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateOwnedOrUpdate(kubeCli KubeClient, obj client.Object, mutateFn func() error) error {
	key := client.ObjectKeyFromObject(obj)
	if err := kubeCli.Get(key, obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err := mutate(mutateFn, key, obj); err != nil {
			return err
		}
		return kubeCli.CreateOwned(obj)
	}

	existing := obj.DeepCopyObject()
	if err := mutate(mutateFn, key, obj); err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(existing, obj) {
		return nil
	}

	if err := kubeCli.Update(obj); err != nil {
		return err
	}
	return nil
}

func mutate(f func() error, key client.ObjectKey, obj client.Object) error {
	if err := f(); err != nil {
		return err
	}
	if newKey := client.ObjectKeyFromObject(obj); key != newKey {
		return fmt.Errorf("MutateFn cannot mutate object name and/or object namespace")
	}
	return nil
}
