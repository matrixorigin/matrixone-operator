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

package v1alpha1

import (
	"github.com/blang/semver/v4"
)

type Gate string

const (
	OperatorVersionAnno = "matrixorigin.io/operator-version"
)

const (
	GateInplaceConfigmapUpdate   Gate = "InplaceConfigmapUpdate"
	GateInplacePoolRollingUpdate Gate = "InplacePoolRollingUpdate"
)

var (
	gateVersions = map[Gate][]semver.Version{
		GateInplaceConfigmapUpdate:   {semver.MustParse("1.3.0")},
		GateInplacePoolRollingUpdate: {semver.MustParse("1.3.0")},
	}
)

func (g Gate) Enabled(v semver.Version) bool {
	for _, minVersion := range gateVersions[g] {
		if versionPrecedes(minVersion, v) {
			return true
		}
	}
	return false
}
