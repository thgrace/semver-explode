package npm

import "testing"

func TestRangeContainsPositive(t *testing.T) {
	cases := []struct {
		rng, ver string
	}{
		{"1.2.3", "1.2.3"},
		{">=1.2.0", "1.5.0"},
		{">=1.2.0 <2.0.0", "1.9.9"},
		{"^1.2.3", "1.9.9"},
		{"^1.2.3", "1.2.3"},
		{"^0.2.3", "0.2.9"},
		{"^0.0.3", "0.0.3"},
		{"~1.2.3", "1.2.9"},
		{"~1.2.3", "1.2.3"},
		{"1.2.x", "1.2.7"},
		{"1.x", "1.99.0"},
		{"*", "5.0.0"},
		{"", "5.0.0"},
		{"5.0.0 - 7.2.3", "6.4.0"},
		{"5.0.0 - 7.2.3", "7.2.3"},
		{"5.0.0 - 7.2.3", "5.0.0"},
		{"~1.6.5 || >=1.7.2", "1.7.5"},
		{"~1.6.5 || >=1.7.2", "1.6.9"},
		{"1 - 2", "1.0.0"},
		{"1 - 2", "2.0.0"},
		{"1 - 2", "1.5.0"},
		{"^1.2.3-beta.2", "1.2.3-beta.4"},
		{"^1.2.3-beta.2", "1.2.3"},
		{"1.2.3 - 2.3", "2.3.0"},
		{"1.2.3 - 2", "2.9.9"},
		{">1.2", "1.3.0"},
		{">=1.2", "1.2.0"},
		{"<1.2", "1.1.99"},
		{"<=1.2", "1.2.99"},
		{"=1.2", "1.2.5"},
		{"=1", "1.0.0"},
		// caret on X-ranges
		{"^1.x", "1.5.0"},
		{"^1.2.x", "1.2.5"},
		{"^1.2.x", "1.9.99"},
		// caret on ^0.0.0 boundary
		{"^0.0.0", "0.0.0"},
		// caret with prerelease at major 0 + minor 0
		{"^0.0.3-beta", "0.0.3-beta.5"},
		{"^0.0.3-beta", "0.0.3"},
		// tilde with prerelease
		{"~1.2.3-beta", "1.2.3-beta.5"},
		{"~1.2.3-beta", "1.2.3"},
		// tilde on single-part
		{"~0", "0.5.0"},
		{"~1", "1.99.99"},
		// prerelease gate on chained range with explicit prerelease bound
		{">=1.2.3-rc <2.0.0", "1.2.3-rc.5"},
	}
	for _, tc := range cases {
		r, err := ParseRange(tc.rng)
		if err != nil {
			t.Fatalf("ParseRange(%q): %v", tc.rng, err)
		}
		v := mustV(t, tc.ver)
		if !r.Contains(v) {
			t.Errorf("Range(%q).Contains(%q) = false, want true", tc.rng, tc.ver)
		}
	}
}

func TestRangeContainsNegative(t *testing.T) {
	cases := []struct {
		rng, ver string
	}{
		{"^1.2.3", "2.0.0"},
		{"^0.2.3", "0.3.0"},
		{"^0.0.3", "0.0.4"},
		{"~1.2.3", "1.3.0"},
		{"^1.2.3", "1.2.4-beta.0"},
		{">=1.0.0", "2.0.0-rc.1"},
		{"*", "1.0.0-rc.1"},
		{"1.2.3", "1.2.4"},
		{"<1.2.3", "1.2.3"},
		{">1.2.3", "1.2.3"},
		{"5.0.0 - 7.2.3", "7.2.4"},
		{"5.0.0 - 7.2.3", "4.9.9"},
		{"1.2.x", "1.3.0"},
		{"^1.2.3-beta.2", "1.2.4-beta.0"},
		// caret on X-ranges
		{"^1.x", "2.0.0"},
		{"^1.2.x", "2.0.0"},
		// caret on ^0.0.0 boundary
		{"^0.0.0", "0.0.1"},
		// caret with prerelease at major 0 + minor 0
		{"^0.0.3-beta", "0.0.4-beta"},
		{"^0.0.3-beta", "0.0.4"},
		// tilde with prerelease
		{"~1.2.3-beta", "1.3.0"},
		// tilde on single-part
		{"~0", "1.0.0"},
		{"~1", "2.0.0"},
		// prerelease gate on chained range
		{">=1.2.3 <2.0.0", "1.5.0-beta.1"},
		// prerelease gate is MMP-specific
		{">=1.2.3-rc <2.0.0", "1.5.0-beta"},
	}
	for _, tc := range cases {
		r, err := ParseRange(tc.rng)
		if err != nil {
			t.Fatalf("ParseRange(%q): %v", tc.rng, err)
		}
		v := mustV(t, tc.ver)
		if r.Contains(v) {
			t.Errorf("Range(%q).Contains(%q) = true, want false", tc.rng, tc.ver)
		}
	}
}

func TestRangeParseErrors(t *testing.T) {
	bad := []string{
		"^^1.2.3",
		">=>2",
		"abc",
		">",
		"~",
		"^",
		"1.2.3 - ",
		" - 1.2.3",
		"!=1.2.3",
		"!= 1.2.3",
		"~>1.2.3",
		"1.",
		"1.2.",
	}
	for _, s := range bad {
		if _, err := ParseRange(s); err == nil {
			t.Errorf("ParseRange(%q) expected error", s)
		}
	}
}

func TestRangeString(t *testing.T) {
	r, err := ParseRange(">=1.2.0 <2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if r.String() != ">=1.2.0 <2.0.0" {
		t.Errorf("String() = %q, want raw input", r.String())
	}
}
