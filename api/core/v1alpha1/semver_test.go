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
	"testing"

	"github.com/blang/semver/v4"

	. "github.com/onsi/gomega"
)

// TestFeatureMapsNoOverlap guards against accidentally placing the same feature in both
// featureGlobalMinVersions and featureVersions. If a feature appears in both maps,
// HasMOFeature() silently takes the global path and the per-major entries become dead code,
// making the behavior hard to reason about.
func TestFeatureMapsNoOverlap(t *testing.T) {
	g := NewGomegaWithT(t)
	for f := range featureGlobalMinVersions {
		_, inPerMajor := featureVersions[f]
		g.Expect(inPerMajor).To(BeFalse(),
			"feature %q is defined in both featureGlobalMinVersions and featureVersions; pick one", f)
	}
}

func TestHasMOFeature(t *testing.T) {
	g := NewGomegaWithT(t)
	g.Expect(HasMOFeature(mustParse("1.1.2"), MOFeaturePipelineInfo)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("1.1.3"), MOFeaturePipelineInfo)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("1.2.0"), MOFeaturePipelineInfo)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("2.0.0"), MOFeaturePipelineInfo)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("1.1.1"), MOFeaturePipelineInfo)).To(BeFalse())
	g.Expect(HasMOFeature(mustParse("1.1.1"), MOFeaturePipelineInfo)).To(BeFalse())
	g.Expect(HasMOFeature(mustParse("1.1.2"), MOFeaturePipelineInfo)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("v1.1.1"), MOFeaturePipelineInfo)).To(BeFalse())
	g.Expect(HasMOFeature(mustParse("v1.1.2-rc1"), MOFeaturePipelineInfo)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("v1.2.0-alpha.1"), MOFeatureLockMigration)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("v1.2.2-woraround-something-else"), MOFeatureLockMigration)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("2.0.1"), MOFeatureLockMigration)).To(BeTrue())
	featureVersions["dummy"] = []semver.Version{mustParse("1.2.3")}
	t.Cleanup(func() { delete(featureVersions, "dummy") })
	g.Expect(HasMOFeature(mustParse("v1.2.3"), "dummy")).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("v1.3.0"), "dummy")).To(BeFalse())
}

// TestHasMOFeature_DiscoveryFixed is a regression test for issue #597.
// MOFeatureDiscoveryFixed is now in featureGlobalMinVersions (cross-major), so it must return
// true for all MO versions >= 2.0.0 regardless of major — including future 4.x, 5.x, etc. —
// without needing a new entry per major version.
func TestHasMOFeature_DiscoveryFixed(t *testing.T) {
	g := NewGomegaWithT(t)
	// MO 2.x — original fix
	g.Expect(HasMOFeature(mustParse("2.0.0"), MOFeatureDiscoveryFixed)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("2.1.0"), MOFeatureDiscoveryFixed)).To(BeTrue())
	// MO 3.x — regression from issue #597
	g.Expect(HasMOFeature(mustParse("3.0.0"), MOFeatureDiscoveryFixed)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("3.0.16"), MOFeatureDiscoveryFixed)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("v3.0.16-bda2d138a-2026-06-24"), MOFeatureDiscoveryFixed)).To(BeTrue())
	// MO 4.x — must work without any new entry in featureGlobalMinVersions
	g.Expect(HasMOFeature(mustParse("4.0.0"), MOFeatureDiscoveryFixed)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("4.5.2"), MOFeatureDiscoveryFixed)).To(BeTrue())
	// MO 1.x — discovery-address not yet supported
	g.Expect(HasMOFeature(mustParse("1.2.0"), MOFeatureDiscoveryFixed)).To(BeFalse())
	g.Expect(HasMOFeature(mustParse("1.9.9"), MOFeatureDiscoveryFixed)).To(BeFalse())
}

// TestHasMOFeature_OtherFeaturesNotExtendedTo3x guards against accidentally widening the
// version gate for features that have NOT been explicitly verified against MO 3.x. Extending
// featureVersions in bulk (i.e. blindly adding "3.0.0" to every feature) would silently flip
// unrelated behavior (lock migration handshake, pipeline/sharding stats collection, session
// source accounting) on MO 3.x without dedicated verification. Only MOFeatureDiscoveryFixed
// has been confirmed compatible with 3.x so far (see #597); this test should be updated
// deliberately, one feature at a time, as each is verified.
func TestHasMOFeature_OtherFeaturesNotExtendedTo3x(t *testing.T) {
	g := NewGomegaWithT(t)
	unverifiedOn3x := []MOFeature{
		MOFeaturePipelineInfo,
		MOFeatureSessionSource,
		MOFeatureLockMigration,
		MOFeatureShardingMigration,
	}
	for _, f := range unverifiedOn3x {
		g.Expect(HasMOFeature(mustParse("3.0.0"), f)).To(BeFalse(), "feature %s should not yet be enabled on MO 3.x", f)
	}
}

func mustParse(s string) semver.Version {
	v, err := semver.ParseTolerant(s)
	if err != nil {
		panic(err)
	}
	return v
}
