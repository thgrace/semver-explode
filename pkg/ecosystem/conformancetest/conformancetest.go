// Package conformancetest provides a shared test suite that verifies any
// ecosystem.Ecosystem implementation satisfies the ecosystem contract.
package conformancetest

import (
	"testing"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
)

// Fixtures holds ecosystem-specific test inputs that the conformance suite
// needs but cannot derive (e.g., a known-good canonical version string).
type Fixtures struct {
	// CanonicalVersion is a version string that round-trips through
	// ParseVersion->String unchanged. Required.
	CanonicalVersion string

	// OrderedVersions is a list of valid version strings in ascending order
	// (each strictly less than the next). Must have at least 3 entries.
	// Used to verify Compare implements a strict total order.
	OrderedVersions []string

	// EmptyRange is a range string that parses to a "match everything"
	// range, if the ecosystem supports one (e.g. "*" for npm, "" for pypi).
	// Set to nil to skip the EmptyRange sub-test entirely.
	// Set EmptyRangeMatchesAll = true if it should match CanonicalVersion.
	EmptyRange           *string
	EmptyRangeMatchesAll bool

	// PrereleaseVersion is an optional prerelease version string. When set,
	// the suite verifies it parses successfully and that IsPrerelease() == true.
	PrereleaseVersion string

	// EmptyRangeRejectsPrereleases, when true, causes the EmptyRange sub-test
	// to additionally assert that the empty range rejects PrereleaseVersion
	// (i.e. r.Contains(pre) == false). When false and EmptyRangeMatchesAll is
	// true, it asserts r.Contains(pre) == true.
	// Ignored when PrereleaseVersion is empty or EmptyRange is nil.
	EmptyRangeRejectsPrereleases bool
}

// Run executes the standard conformance tests against the given ecosystem
// with the provided fixtures. Call from a TestXxx function in a per-ecosystem
// test file, e.g.:
//
//	func TestConformance(t *testing.T) {
//	    conformancetest.Run(t, npm.New(), conformancetest.Fixtures{...})
//	}
func Run(t *testing.T, eco ecosystem.Ecosystem, f Fixtures) {
	t.Helper()

	t.Run("Name_nonempty", func(t *testing.T) {
		t.Helper()
		if eco.Name() == "" {
			t.Errorf("Ecosystem.Name() returned empty string")
		}
	})

	t.Run("ParseVersion_rejects_empty", func(t *testing.T) {
		t.Helper()
		_, err := eco.ParseVersion("")
		if err == nil {
			t.Errorf("ParseVersion(\"\") returned nil error; want non-nil")
		}
	})

	t.Run("ParseVersion_accepts_canonical", func(t *testing.T) {
		t.Helper()
		_, err := eco.ParseVersion(f.CanonicalVersion)
		if err != nil {
			t.Fatalf("ParseVersion(%q) returned unexpected error: %v", f.CanonicalVersion, err)
		}
	})

	t.Run("String_roundtrip", func(t *testing.T) {
		t.Helper()
		v1, err := eco.ParseVersion(f.CanonicalVersion)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", f.CanonicalVersion, err)
		}
		s1 := v1.String()

		v2, err := eco.ParseVersion(s1)
		if err != nil {
			t.Fatalf("ParseVersion(%q) after first String(): %v", s1, err)
		}
		s2 := v2.String()

		if s1 != s2 {
			t.Errorf("String() not stable after round-trip: first=%q second=%q", s1, s2)
		}
	})

	t.Run("Compare_reflexivity", func(t *testing.T) {
		t.Helper()
		for _, vs := range f.OrderedVersions {
			v, err := eco.ParseVersion(vs)
			if err != nil {
				t.Fatalf("ParseVersion(%q): %v", vs, err)
			}
			if got := v.Compare(v); got != 0 {
				t.Errorf("Compare(%q, %q) = %d, want 0 (reflexivity)", vs, vs, got)
			}
		}
	})

	t.Run("Compare_antisymmetry", func(t *testing.T) {
		t.Helper()
		versions := parseAll(t, eco, f.OrderedVersions)
		for i := 0; i+1 < len(versions); i++ {
			a, aStr := versions[i], f.OrderedVersions[i]
			b, bStr := versions[i+1], f.OrderedVersions[i+1]

			if got := a.Compare(b); got >= 0 {
				t.Errorf("Compare(%q, %q) = %d, want < 0 (antisymmetry: a < b)", aStr, bStr, got)
			}
			if got := b.Compare(a); got <= 0 {
				t.Errorf("Compare(%q, %q) = %d, want > 0 (antisymmetry: b > a)", bStr, aStr, got)
			}
		}
	})

	t.Run("Compare_transitivity", func(t *testing.T) {
		t.Helper()
		if len(f.OrderedVersions) < 3 {
			t.Fatalf("OrderedVersions must have at least 3 entries for transitivity check; got %d", len(f.OrderedVersions))
		}
		versions := parseAll(t, eco, f.OrderedVersions)
		// Check all triples i < j < k: a < b && b < c implies a < c.
		for i := 0; i < len(versions); i++ {
			for j := i + 1; j < len(versions); j++ {
				for k := j + 1; k < len(versions); k++ {
					a, aStr := versions[i], f.OrderedVersions[i]
					b, bStr := versions[j], f.OrderedVersions[j]
					c, cStr := versions[k], f.OrderedVersions[k]
					if a.Compare(b) >= 0 || b.Compare(c) >= 0 {
						// antisymmetry already caught this
						continue
					}
					if got := a.Compare(c); got >= 0 {
						t.Errorf("Compare(%q, %q) = %d, want < 0 (transitivity: %q < %q < %q implies %q < %q)",
							aStr, cStr, got, aStr, bStr, cStr, aStr, cStr)
					}
				}
			}
		}
	})

	if f.PrereleaseVersion != "" {
		t.Run("ParseVersion_accepts_prerelease", func(t *testing.T) {
			t.Helper()
			_, err := eco.ParseVersion(f.PrereleaseVersion)
			if err != nil {
				t.Fatalf("ParseVersion(%q) returned unexpected error: %v", f.PrereleaseVersion, err)
			}
		})

		t.Run("IsPrerelease_true_for_prerelease", func(t *testing.T) {
			t.Helper()
			v, err := eco.ParseVersion(f.PrereleaseVersion)
			if err != nil {
				t.Fatalf("ParseVersion(%q): %v", f.PrereleaseVersion, err)
			}
			if !v.IsPrerelease() {
				t.Errorf("ParseVersion(%q).IsPrerelease() = false, want true", f.PrereleaseVersion)
			}
		})
	}

	if f.EmptyRange != nil {
		emptyRange := *f.EmptyRange
		t.Run("EmptyRange", func(t *testing.T) {
			t.Helper()
			r, err := eco.ParseRange(emptyRange)
			if err != nil {
				t.Fatalf("ParseRange(%q): %v", emptyRange, err)
			}

			if f.EmptyRangeMatchesAll {
				v, err := eco.ParseVersion(f.CanonicalVersion)
				if err != nil {
					t.Fatalf("ParseVersion(%q): %v", f.CanonicalVersion, err)
				}
				if !r.Contains(v) {
					t.Errorf("Range(%q).Contains(%q) = false, want true (EmptyRangeMatchesAll)", emptyRange, f.CanonicalVersion)
				}
			}

			if f.PrereleaseVersion != "" {
				pre, err := eco.ParseVersion(f.PrereleaseVersion)
				if err != nil {
					t.Fatalf("ParseVersion(%q): %v", f.PrereleaseVersion, err)
				}
				got := r.Contains(pre)
				if f.EmptyRangeRejectsPrereleases {
					if got {
						t.Errorf("Range(%q).Contains(%q) = true, want false (EmptyRangeRejectsPrereleases)", emptyRange, f.PrereleaseVersion)
					}
				} else if f.EmptyRangeMatchesAll {
					if !got {
						t.Errorf("Range(%q).Contains(%q) = false, want true (EmptyRangeMatchesAll + !EmptyRangeRejectsPrereleases)", emptyRange, f.PrereleaseVersion)
					}
				}
			}
		})
	}
}

// parseAll parses all version strings using eco and returns the resulting
// ecosystem.Version slice. It calls t.Fatalf on the first error.
func parseAll(t *testing.T, eco ecosystem.Ecosystem, strs []string) []ecosystem.Version {
	t.Helper()
	vs := make([]ecosystem.Version, len(strs))
	for i, s := range strs {
		v, err := eco.ParseVersion(s)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", s, err)
		}
		vs[i] = v
	}
	return vs
}
