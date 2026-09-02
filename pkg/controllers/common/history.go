// Copyright 2025 Matrix Origin
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
	"fmt"
	"hash"
	"hash/fnv"

	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/rand"
)

func HashControllerRevision(obj any) (string, error) {
	hf := fnv.New32()
	if obj != nil {
		err := deepHashObject(hf, obj)
		if err != nil {
			return "", err
		}
	}
	return rand.SafeEncodeString(fmt.Sprint(hf.Sum32())), nil
}

func deepHashObject(hasher hash.Hash, objectToWrite interface{}) error {
	hasher.Reset()
	// using json with omitempty tag to avoid hash change caused by newly added (but unset) fields
	s, err := json.Marshal(objectToWrite)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(hasher, s)
	return err
}
