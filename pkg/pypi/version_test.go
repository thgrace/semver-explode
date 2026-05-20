package pypi

import (
	"testing"
)

func mustPV(t *testing.T, s string) Version {
	t.Helper()
	v, err := ParseVersion(s)
	if err != nil {
		t.Fatalf("ParseVersion(%q): %v", s, err)
	}
	return v
}

func TestParseVersionSuccess(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"1.0", "1.0"},
		{"1.0.0", "1.0.0"},
		{"1!2.0", "1!2.0"},
		{"v1.0", "1.0"},
		{" 1.0 ", "1.0"},
		{"1.0a1", "1.0a1"},
		{"1.0.a.1", "1.0a1"},
		{"1.0-alpha1", "1.0a1"},
		{"1.0alpha1", "1.0a1"},
		{"1.0beta", "1.0b0"},
		{"1.0rc1", "1.0rc1"},
		{"1.0c1", "1.0rc1"},
		{"1.0pre1", "1.0rc1"},
		{"1.0preview1", "1.0rc1"},
		{"1.0.post1", "1.0.post1"},
		{"1.0-1", "1.0.post1"},
		{"1.0post1", "1.0.post1"},
		{"1.0.rev1", "1.0.post1"},
		{"1.0.dev0", "1.0.dev0"},
		{"1.0a1.post1.dev1", "1.0a1.post1.dev1"},
		{"1.0+ubuntu1", "1.0+ubuntu1"},
		{"1.0+ubuntu.1", "1.0+ubuntu.1"},
		{"1.0+UBUNTU.1", "1.0+ubuntu.1"},
		{"01.02.03", "1.2.3"},
		{"1.0.0.0.0", "1.0.0.0.0"},
		// Leading zeros stripped in each release segment
		{"01.02", "1.2"},
		// Normalization edge cases
		{"1.0ALPHA1", "1.0a1"},
		{"1.0-pre1", "1.0rc1"},
		{"1.0_alpha_1", "1.0a1"},
		{"1.0+UBUNTU-1", "1.0+ubuntu.1"},
		// rc
		{"1.0a", "1.0a0"},
		// preserve release tuple length
		{"1.0.0", "1.0.0"},
	}
	for _, tc := range cases {
		v, err := ParseVersion(tc.in)
		if err != nil {
			t.Errorf("ParseVersion(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if got := v.String(); got != tc.want {
			t.Errorf("ParseVersion(%q).String() = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseVersionFailure(t *testing.T) {
	bad := []string{
		"",
		"   ",
		"1.0.x",
		"1..0",
		"1.0+",
		"1.0+!",
		"1!",
		"!1.0",
		"abc",
		"1.0+a..b",
	}
	for _, s := range bad {
		if _, err := ParseVersion(s); err == nil {
			t.Errorf("ParseVersion(%q) expected error, got none", s)
		}
	}
}

func TestCompareOrdering(t *testing.T) {
	// Each string parses to a version; the slice is in strictly ascending order.
	ordered := []string{
		"1.0.dev0",
		"1.0a0.dev0",
		"1.0a0",
		"1.0a1",
		"1.0b0",
		"1.0rc0",
		"1.0",
		"1.0.post0.dev0",
		"1.0.post0",
		"1.0.post1",
	}
	vs := make([]Version, len(ordered))
	for i, s := range ordered {
		vs[i] = mustPV(t, s)
	}
	for i := 0; i < len(vs); i++ {
		for j := 0; j < len(vs); j++ {
			got := vs[i].cmp(vs[j])
			var want int
			switch {
			case i < j:
				want = -1
			case i > j:
				want = 1
			default:
				want = 0
			}
			if got != want {
				t.Errorf("cmp(%s, %s) = %d, want %d", ordered[i], ordered[j], got, want)
			}
		}
	}
}

func TestCompareEpoch(t *testing.T) {
	a := mustPV(t, "0!1.0")
	b := mustPV(t, "1!0.0")
	if got := a.cmp(b); got != -1 {
		t.Errorf("epoch: cmp(0!1.0, 1!0.0) = %d, want -1", got)
	}
}

func TestCompareReleaseEquality(t *testing.T) {
	cases := [][2]string{
		{"1.0", "1.0.0"},
		{"1.0", "1.0.0.0"},
		{"1.0.0", "1.0.0.0"},
	}
	for _, tc := range cases {
		a := mustPV(t, tc[0])
		b := mustPV(t, tc[1])
		if got := a.cmp(b); got != 0 {
			t.Errorf("cmp(%s, %s) = %d, want 0", tc[0], tc[1], got)
		}
	}
}

func TestCompareLocal(t *testing.T) {
	// local present > absent
	noLocal := mustPV(t, "1.0")
	withLocal := mustPV(t, "1.0+local")
	if got := noLocal.cmp(withLocal); got != -1 {
		t.Errorf("cmp(1.0, 1.0+local) = %d, want -1", got)
	}

	// numeric local segment > alpha
	numLocal := mustPV(t, "1.0+1")
	alphaLocal := mustPV(t, "1.0+abc")
	if got := numLocal.cmp(alphaLocal); got != 1 {
		t.Errorf("cmp(1.0+1, 1.0+abc) = %d, want 1", got)
	}

	// local numeric ordering
	l0 := mustPV(t, "1.0+local0")
	l1 := mustPV(t, "1.0+local1")
	if got := l0.cmp(l1); got != -1 {
		t.Errorf("cmp(1.0+local0, 1.0+local1) = %d, want -1", got)
	}
}

func TestIsPrerelease(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"1.0", false},
		{"1.0a1", true},
		{"1.0.post1", false},
		{"1.0.dev0", true},
		{"1.0a1.dev0", true},
		{"1.0.post1.dev0", true},
	}
	for _, tc := range cases {
		v := mustPV(t, tc.in)
		if got := v.IsPrerelease(); got != tc.want {
			t.Errorf("IsPrerelease(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestStringCanonicalization(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"1.0ALPHA1", "1.0a1"},
		{"1.0-pre1", "1.0rc1"},
		{"1.0_alpha_1", "1.0a1"},
		{"1.0-1", "1.0.post1"},
		{"1.0+UBUNTU-1", "1.0+ubuntu.1"},
		{"01.02", "1.2"},
		{"1.0.0", "1.0.0"}, // trailing zeros preserved in String
		{"1.0a", "1.0a0"},  // default pre-number is 0
	}
	for _, tc := range cases {
		v, err := ParseVersion(tc.in)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", tc.in, err)
		}
		if got := v.String(); got != tc.want {
			t.Errorf("String(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
