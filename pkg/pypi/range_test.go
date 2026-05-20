package pypi

import "testing"

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
		"~=1.4.5",
		"===1.0",
		"==1.4.*",
		"!=1.4.*",
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

		// ==X.* prefix wildcard
		{"==1.4.*", "1.4", true},
		{"==1.4.*", "1.4.0", true},
		{"==1.4.*", "1.4.99", true},
		{"==1.4.*", "1.4.5a1", true}, // prefix match ignores pre/post/dev
		{"==1.4.*", "1.5", false},
		{"==1.4.*", "2.0", false},

		// !=X.* negated prefix wildcard
		{"!=1.4.*", "1.5", true},
		{"!=1.4.*", "1.4", false},
		{"!=1.4.*", "1.4.99", false},

		// === arbitrary string equality
		{"===1.0", "1.0", true},
		{"===1.0", "1.0.0", false}, // different normalized string
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
