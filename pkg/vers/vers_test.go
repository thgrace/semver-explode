package vers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
	"github.com/thgrace/semver-explode/pkg/npm"
	"github.com/thgrace/semver-explode/pkg/pypi"
)

// fakeVersion is a simple lexicographic-order version for testing.
type fakeVersion struct{ s string }

func (v fakeVersion) String() string     { return v.s }
func (v fakeVersion) IsPrerelease() bool { return false }
func (v fakeVersion) Compare(other ecosystem.Version) int {
	o := other.(fakeVersion)
	if v.s < o.s {
		return -1
	}
	if v.s > o.s {
		return 1
	}
	return 0
}

type fakeEco struct{ name string }

func (e *fakeEco) Name() string { return e.name }
func (e *fakeEco) ParseVersion(s string) (ecosystem.Version, error) {
	return fakeVersion{s}, nil
}
func (e *fakeEco) ParseRange(s string) (ecosystem.Range, error) {
	return nil, nil
}
func (e *fakeEco) Registry() ecosystem.Registry { return &fakeRegistry{} }

type fakeRegistry struct{}

func (r *fakeRegistry) ListVersions(_ context.Context, _ string) ([]ecosystem.Version, error) {
	return nil, nil
}

// ---- Parse success ----

func TestParse_Success(t *testing.T) {
	cases := []struct {
		input    string
		wantTyp  string
		wantLen  int
		wantOp0  Op
		wantVer0 string
	}{
		{
			input:    "vers:npm/>=1.0.0|<2.0.0",
			wantTyp:  "npm",
			wantLen:  2,
			wantOp0:  Ge,
			wantVer0: "1.0.0",
		},
		{
			input:    "vers:npm/*",
			wantTyp:  "npm",
			wantLen:  1,
			wantOp0:  Star,
			wantVer0: "",
		},
		{
			input:    "vers:pypi/>=4.2|<5",
			wantTyp:  "pypi",
			wantLen:  2,
			wantOp0:  Ge,
			wantVer0: "4.2",
		},
		{
			input:    "vers:gem/>=1.0",
			wantTyp:  "gem",
			wantLen:  1,
			wantOp0:  Ge,
			wantVer0: "1.0",
		},
		{
			input:    "vers:npm/1.0.0",
			wantTyp:  "npm",
			wantLen:  1,
			wantOp0:  Eq,
			wantVer0: "1.0.0",
		},
		{
			input:    "vers:npm/=1.0.0",
			wantTyp:  "npm",
			wantLen:  1,
			wantOp0:  Eq,
			wantVer0: "1.0.0",
		},
		{
			input:    "vers:npm/!=1.0.0",
			wantTyp:  "npm",
			wantLen:  1,
			wantOp0:  Ne,
			wantVer0: "1.0.0",
		},
		{
			input:    "vers:npm/>1.0.0",
			wantTyp:  "npm",
			wantLen:  1,
			wantOp0:  Gt,
			wantVer0: "1.0.0",
		},
		{
			input:    "vers:npm/<=1.0.0",
			wantTyp:  "npm",
			wantLen:  1,
			wantOp0:  Le,
			wantVer0: "1.0.0",
		},
		{
			input:    "vers:npm/<1.0.0",
			wantTyp:  "npm",
			wantLen:  1,
			wantOp0:  Lt,
			wantVer0: "1.0.0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			r, err := Parse(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.Type != tc.wantTyp {
				t.Errorf("Type: got %q, want %q", r.Type, tc.wantTyp)
			}
			if len(r.Constraints) != tc.wantLen {
				t.Errorf("len(Constraints): got %d, want %d", len(r.Constraints), tc.wantLen)
			}
			if len(r.Constraints) > 0 {
				if r.Constraints[0].Op != tc.wantOp0 {
					t.Errorf("Constraints[0].Op: got %d, want %d", r.Constraints[0].Op, tc.wantOp0)
				}
				if r.Constraints[0].Version != tc.wantVer0 {
					t.Errorf("Constraints[0].Version: got %q, want %q", r.Constraints[0].Version, tc.wantVer0)
				}
			}
		})
	}
}

// ---- Parse errors ----

func TestParse_Error(t *testing.T) {
	cases := []struct {
		input   string
		wantErr string
	}{
		{"npm/>=1.0", `missing "vers:" prefix`},
		{"vers:>=1.0", "missing '/'"},
		{"vers:/>=1.0", "empty type"},
		{"vers:npm/", "empty constraints"},
		{"vers:n+pm/>=1.0", "invalid type"},
		{"vers:npm/>=1.0 |<2", "whitespace"},
		// Fix 3: Unicode no-break space (U+00A0) must be rejected.
		{"vers:npm/>=1.0 |<2", "whitespace"},
		// Fix 3: ideographic space (U+3000) must be rejected.
		{"vers:npm/>=1.0　|<2", "whitespace"},
		{"vers:npm/|>=1.0", "leading '|'"},
		{"vers:npm/>=1.0|", "trailing '|'"},
		{"vers:npm/>=1.0||<2.0", "double '|'"},
		{"vers:npm/*|>=1.0", "'*' cannot be mixed"},
		{"vers:npm/>=", "empty version literal"},
		// Fix 4: double-equals operator.
		{"vers:npm/==1.0.0", "malformed version literal"},
		// Fix 4: stacked operator prefix.
		{"vers:npm/>=>=1.0", "malformed version literal"},
		{"vers:NPM/>=1.0.0", "invalid type"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			_, err := Parse(tc.input)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// ---- String round-trip ----

func TestRangeString_RoundTrip(t *testing.T) {
	cases := []string{
		"vers:npm/>=1.0.0|<2.0.0",
		"vers:npm/*",
		"vers:pypi/>=4.2|<5",
		"vers:npm/1.0.0",
		"vers:npm/!=1.0.0",
	}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			r, err := Parse(s)
			if err != nil {
				t.Fatalf("Parse(%q): %v", s, err)
			}
			got := r.String()
			if got != s {
				t.Errorf("String(): got %q, want %q", got, s)
			}
		})
	}
}

// ---- Bind success ----

func TestBind_Success(t *testing.T) {
	eco := &fakeEco{name: "npm"}

	r, err := Parse("vers:npm/>=1.0.0|<3.0.0")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	br, err := r.Bind(eco)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if br == nil {
		t.Fatal("Bind returned nil")
	}
}

func TestBind_GolangAlias(t *testing.T) {
	eco := &fakeEco{name: "go"}
	r, err := Parse("vers:golang/>=1.0.0")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err = r.Bind(eco)
	if err != nil {
		t.Fatalf("Bind with golang alias: %v", err)
	}
}

func TestBind_GemAlias(t *testing.T) {
	eco := &fakeEco{name: "rubygems"}
	r, err := Parse("vers:gem/>=1.0.0")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err = r.Bind(eco)
	if err != nil {
		t.Fatalf("Bind with gem alias: %v", err)
	}
}

// ---- Bind errors ----

func TestBind_TypeMismatch(t *testing.T) {
	eco := &fakeEco{name: "pypi"}
	r, err := Parse("vers:npm/>=1.0.0")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err = r.Bind(eco)
	if err == nil {
		t.Fatal("expected error for type mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "does not match ecosystem") {
		t.Errorf("error %q missing expected message", err.Error())
	}
	// Fix 5: sentinel error.
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("expected errors.Is(err, ErrTypeMismatch), got %v", err)
	}
}

func TestBind_NonCanonicalOrder(t *testing.T) {
	eco := &fakeEco{name: "npm"}
	// Constraints in reverse order: <3.0.0 before >=1.0.0 — non-canonical since
	// 1.0.0 < 3.0.0 so >=1.0.0 must come first in sorted order.
	r, err := Parse("vers:npm/<3.0.0|>=1.0.0")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err = r.Bind(eco)
	if err == nil {
		t.Fatal("expected error for non-canonical order, got nil")
	}
	if !strings.Contains(err.Error(), "canonical order") {
		t.Errorf("error %q missing 'canonical order'", err.Error())
	}
	// Fix 5: sentinel error.
	if !errors.Is(err, ErrNonCanonical) {
		t.Errorf("expected errors.Is(err, ErrNonCanonical), got %v", err)
	}
}

func TestBind_ContradictoryComparators(t *testing.T) {
	eco := &fakeEco{name: "npm"}
	// >=1.0.0 followed by >=2.0.0 — two opens without a close.
	r, err := Parse("vers:npm/>=1.0.0|>=2.0.0")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err = r.Bind(eco)
	if err == nil {
		t.Fatal("expected error for contradictory comparators, got nil")
	}
	if !strings.Contains(err.Error(), "contradictory") {
		t.Errorf("error %q missing 'contradictory'", err.Error())
	}
	// Fix 5: sentinel error.
	if !errors.Is(err, ErrContradictory) {
		t.Errorf("expected errors.Is(err, ErrContradictory), got %v", err)
	}
}

func TestBind_DuplicateVersions(t *testing.T) {
	cases := []struct {
		name string
		vers string
		eco  ecosystem.Ecosystem
	}{
		{
			name: "same version different comparator",
			vers: "vers:npm/>=1.0.0|<1.0.0",
			eco:  npm.New(),
		},
		{
			name: "normalized npm version",
			vers: "vers:npm/1.0.0|v1.0.0",
			eco:  npm.New(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Parse(tc.vers)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.vers, err)
			}
			_, err = r.Bind(tc.eco)
			if err == nil {
				t.Fatal("expected duplicate version error, got nil")
			}
			if !strings.Contains(err.Error(), "duplicate version") {
				t.Errorf("error %q missing 'duplicate version'", err.Error())
			}
			if !errors.Is(err, ErrContradictory) {
				t.Errorf("expected errors.Is(err, ErrContradictory), got %v", err)
			}
		})
	}
}

func TestBind_InvalidComparatorAdjacency(t *testing.T) {
	cases := []string{
		"vers:npm/=1.0.0|<2.0.0",
		"vers:npm/>=1.0.0|=2.0.0",
		"vers:npm/>=1.0.0|!=1.5.0|=2.0.0",
	}

	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			r, err := Parse(input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", input, err)
			}
			_, err = r.Bind(npm.New())
			if err == nil {
				t.Fatal("expected comparator adjacency error, got nil")
			}
			if !errors.Is(err, ErrContradictory) {
				t.Errorf("expected errors.Is(err, ErrContradictory), got %v", err)
			}
		})
	}
}

func TestBind_ValidComparatorAdjacency(t *testing.T) {
	cases := []string{
		"vers:npm/<1.0.0|=2.0.0",
		"vers:npm/<1.0.0|!=1.5.0|=2.0.0",
	}

	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			r, err := Parse(input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", input, err)
			}
			if _, err := r.Bind(npm.New()); err != nil {
				t.Fatalf("Bind(%q): %v", input, err)
			}
		})
	}
}

func TestBind_PyPIEpochVersion(t *testing.T) {
	r, err := Parse("vers:pypi/>=1%214.0")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := r.Constraints[0].Version; got != "1!4.0" {
		t.Fatalf("decoded version = %q, want %q", got, "1!4.0")
	}
	if got := r.String(); got != "vers:pypi/>=1%214.0" {
		t.Fatalf("String() = %q, want %q", got, "vers:pypi/>=1%%214.0")
	}
	if _, err := r.Bind(pypi.New()); err != nil {
		t.Fatalf("Bind: %v", err)
	}
}

// Fix 6: alias whose target ecosystem is not registered must return ErrUnsupportedType.
// We inject a temporary alias entry pointing at a name that is not registered
// in the global ecosystem registry, then bind it against a *different* fake eco
// so the alias code path (not the direct-name path) is exercised.
func TestBind_AliasTargetNotRegistered(t *testing.T) {
	typeAliases["fakealiasXYZ"] = "unregistered-eco-XYZ"
	defer delete(typeAliases, "fakealiasXYZ")

	// eco.Name() is "npm" — doesn't match the alias target "unregistered-eco-XYZ",
	// so resolveType will attempt Lookup("unregistered-eco-XYZ"), which fails,
	// and returns ErrUnsupportedType.
	r := Range{
		Type:        "fakealiasXYZ",
		Constraints: []Constraint{{Op: Ge, Version: "1.0.0"}},
	}
	eco := &fakeEco{name: "npm"}
	_, err := r.Bind(eco)
	if err == nil {
		t.Fatal("expected error for unregistered alias target, got nil")
	}
	if !errors.Is(err, ErrUnsupportedType) {
		t.Errorf("expected errors.Is(err, ErrUnsupportedType), got %v", err)
	}
}

// ---- Contains ----

func TestContains(t *testing.T) {
	eco := &fakeEco{name: "npm"}

	cases := []struct {
		name  string
		vers  string
		check string
		want  bool
	}{
		{"star matches all", "vers:npm/*", "99.99.99", true},
		{"ge-lt in range", "vers:npm/>=1.0.0|<2.0.0", "1.5.0", true},
		{"ge-lt below", "vers:npm/>=1.0.0|<2.0.0", "0.9.0", false},
		{"ge-lt at boundary ge", "vers:npm/>=1.0.0|<2.0.0", "1.0.0", true},
		{"ge-lt at boundary lt", "vers:npm/>=1.0.0|<2.0.0", "2.0.0", false},
		{"eq exact match", "vers:npm/1.0.0", "1.0.0", true},
		{"eq no match", "vers:npm/1.0.0", "1.0.1", false},
		{"ne excludes", "vers:npm/!=1.0.0", "1.0.0", false},
		// Fix 2: lone != should pass versions that don't match.
		{"ne allows others", "vers:npm/!=1.0.0", "1.0.1", true},
		{"ge open ended", "vers:npm/>=1.0.0", "5.0.0", true},
		{"ge open below", "vers:npm/>=1.0.0", "0.9.0", false},
		// Fix 2: lone <= and < should match versions below the bound.
		{"le open ended", "vers:npm/<=2.0.0", "1.0.0", true},
		{"lt below bound", "vers:npm/<2.0.0", "1.0.0", true},

		// Additional cases (Fix 2).
		{"ne exact", "vers:npm/!=1.0.0", "1.0.0", false},
		{"lt below", "vers:npm/<1.0.0", "0.5.0", true},
		{"lt at bound", "vers:npm/<1.0.0", "1.0.0", false},
		{"gt at bound", "vers:npm/>1.0.0", "1.0.0", false},
		{"gt above", "vers:npm/>1.0.0", "2.0.0", true},

		// Disjoint union: >=1|<2|>=3|<4
		{"disjoint union first band", "vers:npm/>=1|<2|>=3|<4", "1.5.0", true},
		{"disjoint union gap", "vers:npm/>=1|<2|>=3|<4", "2.5.0", false},
		{"disjoint union second band", "vers:npm/>=1|<2|>=3|<4", "3.5.0", true},

		// Discrete equality list.
		{"eq list match first", "vers:npm/=1.0.0|=2.0.0", "1.0.0", true},
		{"eq list no match mid", "vers:npm/=1.0.0|=2.0.0", "1.5.0", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Parse(tc.vers)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.vers, err)
			}
			br, err := r.Bind(eco)
			if err != nil {
				t.Fatalf("Bind: %v", err)
			}
			v := fakeVersion{tc.check}
			got := br.Contains(v)
			if got != tc.want {
				t.Errorf("Contains(%q) = %v, want %v", tc.check, got, tc.want)
			}
		})
	}
}

func TestContains_EmptyRange(t *testing.T) {
	br := &boundRange{eco: &fakeEco{name: "npm"}}
	if br.Contains(fakeVersion{"1.0.0"}) {
		t.Error("empty range should not contain anything")
	}
}

func TestContains_Star(t *testing.T) {
	br := &boundRange{eco: &fakeEco{name: "npm"}, star: true}
	if !br.Contains(fakeVersion{"anything"}) {
		t.Error("star range should contain everything")
	}
}
