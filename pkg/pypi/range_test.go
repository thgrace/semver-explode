package pypi

import (
	"testing"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
)

type arbitraryVersion struct {
	value      string
	prerelease bool
}

func (v arbitraryVersion) String() string { return v.value }

func (v arbitraryVersion) Compare(ecosystem.Version) int { return 0 }

func (v arbitraryVersion) IsPrerelease() bool { return v.prerelease }

func mustVer(t *testing.T, s string) Version {
	t.Helper()
	v, err := ParseVersion(s)
	if err != nil {
		t.Fatalf("ParseVersion(%q): %v", s, err)
	}
	return v
}

func mustRange(t *testing.T, s string) Range {
	t.Helper()
	r, err := ParseRange(s)
	if err != nil {
		t.Fatalf("ParseRange(%q): %v", s, err)
	}
	return r
}

func TestParseRangeSuccess(t *testing.T) {
	cases := []string{
		"==1.0",
		"!=1.0",
		"<1.0",
		"<=1.0",
		">1.0",
		">=1.0",
		"~=1.0",
		"~=1!1.0",
		"~=1.4.5",
		"===1.0",
		"===foobar",
		"==1.4.*",
		"==1!1.4.*",
		"!=1.4.*",
		"!=1!1.4.*",
		">=1.0,<2.0",
		">= 1.0 , < 2.0",
		">=1.0, !=1.5, <2.0",
		"",
		"~=1.0.4.5",
	}
	for _, s := range cases {
		if _, err := ParseRange(s); err != nil {
			t.Errorf("ParseRange(%q) unexpected error: %v", s, err)
		}
	}
}

func TestParseRangeFailure(t *testing.T) {
	cases := []string{
		"~=1",     // single segment
		"==",      // no version
		">>1.0",   // bad op
		"1.0",     // missing op
		">=1.0,<", // trailing empty specifier
		"==1.0a1.*",
		"==1.0.post1.*",
		"==1.0.dev1.*",
		"==1.0.dev0.*",
		"==1.0+local.*",
		// Local versions are rejected for ~= and all ordered comparators.
		"~=1.0+local",
		">=1.0+local",
		">1.0+local",
		"<=1.0+local",
		// Whitespace inside a multi-character operator is not allowed.
		"> = 1.0",
	}
	for _, s := range cases {
		if _, err := ParseRange(s); err == nil {
			t.Errorf("ParseRange(%q) expected error, got nil", s)
		}
	}
}

func TestRangeString(t *testing.T) {
	r := mustRange(t, ">=1.0,<2.0")
	if r.String() != ">=1.0,<2.0" {
		t.Errorf("String() = %q, want %q", r.String(), ">=1.0,<2.0")
	}
}

func TestRangeContains(t *testing.T) {
	cases := []struct {
		rng   string
		ver   string
		match bool
	}{
		// == exact
		{"==1.0", "1.0", true},
		{"==1.0", "1.0.0", true}, // trailing zero equiv
		{"==1.0", "1.0.1", false},
		{"==1.0", "1.0a1", false}, // pre-release of same release but not equal
		// == local stripping
		{"==1.0", "1.0+ubuntu1", true}, // constraint has no local → ignore candidate local
		{"==1.0+ubuntu", "1.0+ubuntu", true},
		{"==1.0+ubuntu", "1.0", false},
		{"==1.0+ubuntu", "1.0+debian", false},

		// != (inverse of ==)
		{"!=1.0", "1.0", false},
		{"!=1.0", "1.0.0", false},
		{"!=1.0", "1.1", true},
		{"!=1.0", "1.0+ubuntu1", false}, // same local-stripping rule
		{"!=1.0a1", "1.0a2", false},     // exclusion-only prerelease does not opt in

		// < exclusive bound
		{"<1.0", "0.9", true},
		{"<1.0", "0.9.post5", true},
		{"<1.0", "1.0", false},       // excluded: same release
		{"<1.0", "1.0a1", false},     // excluded: same release (pre)
		{"<1.0", "1.0.post1", false}, // excluded: same release (post)
		{"<1.0", "1.0.dev0", false},  // excluded: same release (dev)
		{"<1.0", "1.1", false},

		// <= no special rules
		{"<=1.0", "1.0", true},
		{"<=1.0", "0.9", true},
		{"<=1.0", "1.0.post1", false}, // post is greater than 1.0

		// > exclusive bound
		{">1.0", "1.1", true},
		{">1.0", "2.0", true},
		{">1.0", "1.0", false},       // excluded: same release
		{">1.0", "1.0a1", false},     // excluded: same release (pre)
		{">1.0", "1.0.post1", false}, // excluded: same release (post)
		{">1.0", "0.9", false},

		// >= no special rules
		{">=1.0", "1.0", true},
		{">=1.0", "1.0.post1", true},
		{">=1.0", "1.5", true},
		{">=1.0", "1.5a1", false}, // prerelease filter
		{">=1.0", "0.9", false},

		// prerelease filter — set has prerelease → allow
		{">=1.0a1", "1.0a1", true},
		{">=1.0a1", "1.5a1", true},
		{">=1.0a1", "1.5", true},

		// ~= compatible release
		{"~=1.4", "1.4", true},
		{"~=1.4", "1.4.5", true},
		{"~=1.4", "1.99", true},
		{"~=1.4", "2.0", false},
		{"~=1.4", "1.3", false},
		{"~=1.4.5", "1.4.5", true},
		{"~=1.4.5", "1.4.99", true},
		{"~=1.4.5", "1.5", false},
		{"~=1.4.5", "1.4.4", false},
		{"~=1.0.0", "1.0", true},
		{"~=1.0.0", "1.0.5", true},
		{"~=1.0.0", "1.1", false}, // trailing zero controls compatibility width
		{"~=1.4.5.0", "1.4.5.9", true},
		{"~=1.4.5.0", "1.4.6", false},
		{"~=1!1.0", "1!1.5", true},
		{"~=1!1.0", "2!1.5", false},

		// ==X.* prefix wildcard
		{"==1.4.*", "1.4", true},
		{"==1.4.*", "1.4.0", true},
		{"==1.4.*", "1.4.99", true},
		{"==1.4.*", "1.4.post1", true},
		{"==1.4.*", "1.4.5a1", false}, // default Contains excludes prereleases
		{">=1.4a1,==1.4.*", "1.4a1", true},
		{"==1.4.*", "1.5", false},
		{"==1.4.*", "2.0", false},
		{"==1!1.4.*", "1!1.4.5", true},
		{"==1!1.4.*", "1!1.5", false},
		{"==1!1.4.*", "1.4.5", false},

		// !=X.* negated prefix wildcard
		{"!=1.4.*", "1.5", true},
		{"!=1.4.*", "1.4", false},
		{"!=1.4.*", "1.4.99", false},
		{"!=1!1.4.*", "1!1.5", true},
		{"!=1!1.4.*", "1!1.4.5", false},

		// === arbitrary string equality
		{"===1.0", "1.0", true},
		{"===1.0", "1.0.0", false}, // different normalized string
		{"===v1.0", "1.0", false},  // no version normalization for ===
		{"===1.0+LOCAL", "1.0+local", true},
		{"===1.0a1", "1.0a1", true},
		{"===1.0", "1.1", false},

		// AND combination
		{">=1.0,<2.0", "1.0", true},
		{">=1.0,<2.0", "1.5", true},
		{">=1.0,<2.0", "1.99", true},
		{">=1.0,<2.0", "0.9", false},
		{">=1.0,<2.0", "2.0", false},
		{">=1.0,<2.0", "2.0a1", false}, // prerelease filter (>=1.0 has no pre)

		// whitespace is ok
		{">= 1.0 , < 2.0", "1.5", true},
		{">= 1.0 , < 2.0", "2.5", false},

		// empty range matches everything except prereleases
		{"", "1.0", true},
		{"", "1.0a1", false},

		// dev counts as prerelease → filtered
		{">=1.0", "1.0.dev0", false},
		// set explicitly has dev → allow
		{">=1.0.dev0", "1.0.dev0", true},
		{">=1.0.dev0", "1.0a1", true},
		{">=1.0.dev0", "1.0", true},

		// three-part specifier
		{">=1.0, !=1.5, <2.0", "1.4", true},
		{">=1.0, !=1.5, <2.0", "1.5", false},
		{">=1.0, !=1.5, <2.0", "2.0", false},

		// ~= with prerelease in spec: opts the set into prereleases and
		// the compatible-prefix is still all-but-last release segments.
		{"~=1.4.5a1", "1.4.5a1", true},
		{"~=1.4.5a1", "1.4.5b1", true},
		{"~=1.4.5a1", "1.4.99", true},
		{"~=1.4.5a1", "1.5", false},

		// Wildcard with epoch: mismatched epoch never matches.
		{"==1.4.*", "2!1.4", false},
		{"==2!1.4.*", "2!1.4.0", true},
		{"==2!1.4.*", "1.4.0", false},

		// == with local: when constraint has a local, candidate local must
		// match exactly (different local segment counts are not equal).
		{"==1.0+ubuntu", "1.0+ubuntu.1", false},

		// === literal: candidate string must equal spec string after ASCII
		// fold; normalization is NOT applied to the spec side.
		{"===1.0.0", "1.0", false},

		// Multi-== set is parseable but logically empty; no version
		// satisfies both ==1.0 and ==2.0 simultaneously.
		{"==1.0,==2.0", "1.0", false},
		{"==1.0,==2.0", "2.0", false},
	}

	for _, tc := range cases {
		r := mustRange(t, tc.rng)
		v := mustVer(t, tc.ver)
		got := r.Contains(v)
		if got != tc.match {
			t.Errorf("Range(%q).Contains(%q) = %v, want %v", tc.rng, tc.ver, got, tc.match)
		}
	}
}

func TestArbitraryEqualityContainsRawVersion(t *testing.T) {
	r := mustRange(t, "===FooBar")
	if !r.Contains(arbitraryVersion{value: "foobar"}) {
		t.Errorf("Range(%q).Contains(%q) = false, want true", r.String(), "foobar")
	}
	if r.Contains(arbitraryVersion{value: "foo"}) {
		t.Errorf("Range(%q).Contains(%q) = true, want false", r.String(), "foo")
	}

	pre := mustRange(t, "===1.0a1")
	if !pre.Contains(arbitraryVersion{value: "1.0A1", prerelease: true}) {
		t.Errorf("Range(%q).Contains(%q) = false, want true", pre.String(), "1.0A1")
	}

	mixed := mustRange(t, "===FooBar,>=1.0")
	if mixed.Contains(arbitraryVersion{value: "foobar"}) {
		t.Errorf("Range(%q).Contains(%q) = true, want false", mixed.String(), "foobar")
	}
}
