package pypi_test

import (
	"testing"

	"github.com/thgrace/semver-explode/pkg/ecosystem/conformancetest"
	"github.com/thgrace/semver-explode/pkg/pypi"
)

func TestConformance(t *testing.T) {
	conformancetest.Run(t, pypi.New(), conformancetest.Fixtures{
		CanonicalVersion: "1.2.3",
		// OrderedVersions exercises PEP 440 ordering layers:
		//   dev (no-pre/no-post + dev) → NegativeInfinity pre-key → sorts below pre-releases
		//   pre-releases (a < b < rc) → sort below the bare release
		//   post → sorts above the bare release
		//   epoch N! dominates all epoch-0 versions regardless of release
		OrderedVersions: []string{
			"1.0.0",
			"1.2.3.dev1",
			"1.2.3a1",
			"1.2.3rc1",
			"1.2.3",
			"1.2.3.post1",
			"2.0.0",
			"2!0.0.1",
		},
		EmptyRange:                   func() *string { s := ""; return &s }(),
		EmptyRangeMatchesAll:         true,
		PrereleaseVersion:            "1.0.0a1",
		EmptyRangeRejectsPrereleases: true, // pypi empty set rejects prereleases per PEP 440
	})
}
