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

import "github.com/blang/semver/v4"

type MOFeature string

const (
	MOFeaturePipelineInfo  MOFeature = "PipelineInfo"
	MOFeatureSessionSource MOFeature = "SessionSource"
	MOFeatureLockMigration MOFeature = "LockMigration"

	MOFeatureDiscoveryFixed    MOFeature = "DiscoveryFixed"
	MOFeatureShardingMigration MOFeature = "ShardingMigration"
)

var (
	// featureVersions lists the minimum version per major branch from which the feature is
	// available. versionPrecedes() uses major-version isolation, so every new major branch that
	// should retain a feature needs its own explicit entry. Do NOT bulk-add a new major version
	// to every feature "for consistency" — each feature must be verified independently against
	// the new major branch before its gate is extended, to avoid unintentionally flipping
	// behavior (e.g. lock migration handshake, pipeline/sharding stats collection).
	featureVersions = map[MOFeature][]semver.Version{
		MOFeaturePipelineInfo:      {semver.MustParse("1.1.2"), semver.MustParse("1.2.0"), semver.MustParse("2.0.0")},
		MOFeatureSessionSource:     {semver.MustParse("1.1.2"), semver.MustParse("1.2.0"), semver.MustParse("2.0.0")},
		MOFeatureLockMigration:     {semver.MustParse("1.2.0"), semver.MustParse("2.0.0")},
		MOFeatureShardingMigration: {semver.MustParse("2.0.0")},
	}

	// featureGlobalMinVersions lists features that are stable across all future major versions
	// once introduced. Unlike featureVersions, these do NOT need a new entry per major version
	// because the underlying mechanism is a stable config field / protocol that MO guarantees
	// to keep indefinitely. Only add features here when you are confident they will never be
	// removed or incompatibly changed in future major versions.
	featureGlobalMinVersions = map[MOFeature]semver.Version{
		// discovery-address is a stable [hakeeper-client] toml field supported since MO 2.0.
		// It relies on K8s Service routing (operator-side), not on any MO-internal protocol
		// that could change between major versions, so no per-major re-verification is needed.
		MOFeatureDiscoveryFixed: semver.MustParse("2.0.0"),
	}

	MinimalVersion = semver.Version{Major: 0, Minor: 0, Patch: 0}
)

// HasMOFeature returns whether a version contains certain MO feature.
// It checks featureGlobalMinVersions first (cross-major stable features), then
// featureVersions (per-major-verified features).
func HasMOFeature(v semver.Version, f MOFeature) bool {
	if minVer, ok := featureGlobalMinVersions[f]; ok && versionPrecedesCrossMajor(minVer, v) {
		return true
	}
	for _, minVersion := range featureVersions[f] {
		if versionPrecedes(minVersion, v) {
			return true
		}
	}
	return false
}

// versionPrecedes returns whether current version is a strict preceding version of base version
// within the same major. Different major versions have no preceding relationship here, so every
// new major branch requires its own explicit entry in featureVersions.
// Example: 1.2.1 precedes 1.1.0, but 2.1.0 does NOT precede 1.1.0.
func versionPrecedes(baseVersion semver.Version, current semver.Version) bool {
	if baseVersion.Major != current.Major {
		// different major version has no preceding relationship
		return false
	}
	// e.g: 1.2.0, then 1.2.1 and 1.3.1 must be the following versions
	if baseVersion.Patch == 0 && current.Minor >= baseVersion.Minor {
		return true
	}
	return baseVersion.Minor == current.Minor && current.Patch >= baseVersion.Patch
}

// versionPrecedesCrossMajor is like versionPrecedes but without major-version isolation.
// Use this only for stable config fields / protocols guaranteed never to be removed across
// future major versions (see featureGlobalMinVersions).
func versionPrecedesCrossMajor(baseVersion semver.Version, current semver.Version) bool {
	if current.Major != baseVersion.Major {
		return current.Major > baseVersion.Major
	}
	if baseVersion.Patch == 0 && current.Minor >= baseVersion.Minor {
		return true
	}
	return baseVersion.Minor == current.Minor && current.Patch >= baseVersion.Patch
}
