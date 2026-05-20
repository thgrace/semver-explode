package npm

import "testing"

func TestParseVersionRoundTrip(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"1.2.3", "1.2.3"},
		{"v1.2.3", "1.2.3"},
		{"1.2.3-alpha.1+build.7", "1.2.3-alpha.1+build.7"},
		{"0.0.0", "0.0.0"},
		{"10.20.30-rc.1", "10.20.30-rc.1"},
		{"1.2.3-0", "1.2.3-0"},
		{"1.0.0-x-y-z", "1.0.0-x-y-z"},
		{"1.2.3+001", "1.2.3+001"},
		{"1.2.3+0.0", "1.2.3+0.0"},
		{"1.2.3+build.7", "1.2.3+build.7"},
	}
	for _, tc := range cases {
		v, err := ParseVersion(tc.in)
		if err != nil {
			t.Fatalf("ParseVersion(%q) error: %v", tc.in, err)
		}
		if got := v.String(); got != tc.want {
			t.Errorf("ParseVersion(%q).String() = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseVersionPrereleaseBuild(t *testing.T) {
	v, err := ParseVersion("1.2.3-alpha.1+build.7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.IsPrerelease() {
		t.Error("expected prerelease")
	}
	if len(v.Prerelease) != 2 || v.Prerelease[0] != "alpha" || v.Prerelease[1] != "1" {
		t.Errorf("prerelease wrong: %#v", v.Prerelease)
	}
	if len(v.Build) != 2 || v.Build[0] != "build" || v.Build[1] != "7" {
		t.Errorf("build wrong: %#v", v.Build)
	}
}

func mustV(t *testing.T, s string) Version {
	t.Helper()
	v, err := ParseVersion(s)
	if err != nil {
		t.Fatalf("ParseVersion(%q): %v", s, err)
	}
	return v
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.3", "1.2.4", -1},
		{"1.2.3", "1.2.3", 0},
		{"1.2.4", "1.2.3", 1},
		{"1.2.3-alpha", "1.2.3", -1},
		{"1.2.3-alpha.1", "1.2.3-alpha.2", -1},
		{"1.2.3-1", "1.2.3-alpha", -1},
		{"1.2.3-alpha", "1.2.3-alpha.1", -1},
		{"1.2.3+a", "1.2.3+b", 0},
	}
	for _, tc := range cases {
		a := mustV(t, tc.a)
		b := mustV(t, tc.b)
		got := a.cmp(b)
		if got != tc.want {
			t.Errorf("cmp(%s, %s) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestParseVersionErrors(t *testing.T) {
	bad := []string{
		"",
		"1",
		"1.2",
		"1.2.3.4",
		"01.2.3",
		"1.2.3-α",
		"abc",
		"V1.2.3",
		"vv1.2.3",
		"1.2.3-",
		"1.2.3+",
		"1.2.3-a.",
		"1.2.3-a..b",
		"1.2.3+a.",
		"1.2.3+a..b",
		"1.2.3-+build",
	}
	for _, s := range bad {
		if _, err := ParseVersion(s); err == nil {
			t.Errorf("ParseVersion(%q) expected error", s)
		}
	}
}
