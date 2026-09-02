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

package convertor

import (
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/util/errors"
)

type Convertable interface{}

type ConvertFn func(*Convertable, *Convertable, conversion.Scope) error

func Convert[inT, outT Convertable](in *inT, convertFn func(*inT, *outT, conversion.Scope) error) (*outT, error) {
	out := new(outT)
	err := convertFn(in, out, nil)
	return out, err
}

func ConvertSlice[inT, outT Convertable](inSlice []inT, convertFn func(*inT, *outT, conversion.Scope) error) ([]outT, error) {
	outSlice := make([]outT, 0, len(inSlice))
	var errs []error

	for _, item := range inSlice {
		in, out := item, new(outT)
		err := convertFn(&in, out, nil)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		outSlice = append(outSlice, *out)
	}
	return outSlice, errors.NewAggregate(errs)
}
