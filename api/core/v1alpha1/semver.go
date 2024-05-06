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

package v1alpha1

import "github.com/blang/semver/v4"

type MOFeature string

const (
	MOFeaturePipelineInfo  MOFeature = "PipelineInfo"
	MOFeatureSessionSource MOFeature = "SessionSource"
	MOFeatureLockMigration MOFeature = "LockMigration"
)

var (
	featureVersions = map[MOFeature][]semver.Version{
		MOFeaturePipelineInfo:  {semver.MustParse("1.1.2"), semver.MustParse("1.2.0")},
		MOFeatureSessionSource: {semver.MustParse("1.1.2"), semver.MustParse("1.2.0")},
		MOFeatureLockMigration: {semver.MustParse("1.1.4"), semver.MustParse("1.2.0")},
	}

	MinimalVersion = semver.Version{Major: 0, Minor: 0, Patch: 0}
)

// HasMOFeature returns whether a version contains certain MO feature
func HasMOFeature(v semver.Version, f MOFeature) bool {
	for _, minVersion := range featureVersions[f] {
		if versionPrecedes(minVersion, v) {
			return true
		}
	}
	return false
}

// versionPrecedes returns whether current version is a strict preceding version of base version.
// for example, 1.2.1 is a strict preceding version of 1.1.0, but not 1.1.1
func versionPrecedes(baseVersion semver.Version, current semver.Version) bool {
	if baseVersion.Major != current.Major {
		// different major version has no preceding relationship
		return false
	}
	if baseVersion.Patch == 0 {
		return current.GTE(baseVersion)
	}
	return baseVersion.Minor == current.Minor && current.Patch >= baseVersion.Patch
}
