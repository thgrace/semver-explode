package npm_test

import (
	"testing"

	"github.com/thgrace/semver-explode/pkg/ecosystem/conformancetest"
	"github.com/thgrace/semver-explode/pkg/npm"
)

func TestConformance(t *testing.T) {
	conformancetest.Run(t, npm.New(), conformancetest.Fixtures{
		CanonicalVersion: "1.2.3",
		// OrderedVersions exercises semver-2 prerelease ordering rules:
		//   numeric idents sort lower than alphanumeric (alpha < alpha.1 because
		//   len-tiebreak: alpha has 1 ident, alpha.1 has 2 after equal prefix alpha),
		//   and any prerelease sorts below its parent release (rc.1 < 1.0.0).
		//
		// Note: build metadata (e.g. "1.2.3+build.1") is IGNORED in Compare, so a
		// build-metadata-only difference produces Compare==0 rather than strict
		// less-than. Such pairs cannot appear in OrderedVersions; build-metadata
		// isolation is a separate test concern not covered here.
		OrderedVersions: []string{
			"0.9.0",
			"1.0.0-alpha",
			"1.0.0-alpha.1",
			"1.0.0-beta.2",
			"1.0.0-rc.1",
			"1.0.0",
			"1.2.3",
			"2.0.0",
		},
		EmptyRange:                   func() *string { s := "*"; return &s }(),
		EmptyRangeMatchesAll:         true,
		PrereleaseVersion:            "1.0.0-alpha.1",
		EmptyRangeRejectsPrereleases: true, // npm * skips opAny in prereleaseGateAllows → rejects
	})
}
