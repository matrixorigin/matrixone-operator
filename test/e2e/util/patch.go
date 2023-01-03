// Copyright 2023 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package util

import (
	"context"
	"fmt"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Patch(ctx context.Context, cli client.Client, obj client.Object, mutateFn func() error, opts ...client.PatchOption) error {
	key := client.ObjectKeyFromObject(obj)
	if err := cli.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		return err
	}
	before := obj.DeepCopyObject().(client.Object)
	if err := mutateFn(); err != nil {
		return err
	}
	if newKey := client.ObjectKeyFromObject(obj); key != newKey {
		return fmt.Errorf("MutateFn cannot mutate object name and/or object namespace")
	}
	if reflect.DeepEqual(before, obj) {
		// no change to patch
		return nil
	}
	return cli.Patch(ctx, obj, client.MergeFrom(before), opts...)
}
