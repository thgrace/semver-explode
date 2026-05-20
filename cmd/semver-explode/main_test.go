package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
	"github.com/thgrace/semver-explode/pkg/resolve"
)

// fakeVersion is a minimal ecosystem.Version for testing.
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

// fakeRange matches versions whose string equals one of the allowed values.
type fakeRange struct{ allowed []string }

func (r fakeRange) Contains(v ecosystem.Version) bool {
	for _, a := range r.allowed {
		if v.String() == a {
			return true
		}
	}
	return false
}
func (r fakeRange) String() string { return strings.Join(r.allowed, "|") }

// fakeRegistry holds an in-memory list of versions.
type fakeRegistry struct{ versions []ecosystem.Version }

func (r *fakeRegistry) ListVersions(_ context.Context, _ string) ([]ecosystem.Version, error) {
	return r.versions, nil
}

// fakeEco is a minimal ecosystem.Ecosystem backed by in-memory data.
type fakeEco struct {
	name     string
	registry *fakeRegistry
	// rangeAllowed lists versions the range "match" expression will match.
	rangeAllowed []string
}

func (e *fakeEco) Name() string { return e.name }

func (e *fakeEco) ParseVersion(s string) (ecosystem.Version, error) {
	return fakeVersion{s}, nil
}

func (e *fakeEco) ParseRange(s string) (ecosystem.Range, error) {
	return fakeRange{allowed: e.rangeAllowed}, nil
}

func (e *fakeEco) Registry() ecosystem.Registry { return e.registry }

// fakeLookup builds a lookup func from a name→Ecosystem map.
func fakeLookup(m map[string]ecosystem.Ecosystem) func(string) (ecosystem.Ecosystem, bool) {
	return func(name string) (ecosystem.Ecosystem, bool) {
		eco, ok := m[name]
		return eco, ok
	}
}

// ---- parseRequest tests ----

func TestParseRequest(t *testing.T) {
	type tc struct {
		name    string
		args    []string
		wantErr string
		// zero values mean "don't check"
		wantEco        string
		wantPkg        string
		wantRangeExpr  string
		wantPinVersion string
	}

	cases := []tc{
		{
			name:          "legacy 3-arg",
			args:          []string{"npm", "lodash", "^4.17.0"},
			wantEco:       "npm",
			wantPkg:       "lodash",
			wantRangeExpr: "^4.17.0",
		},
		{
			name:          "purl no version with range arg",
			args:          []string{"pkg:npm/lodash", "^4.17.0"},
			wantEco:       "npm",
			wantPkg:       "lodash",
			wantRangeExpr: "^4.17.0",
		},
		{
			name:           "purl with @version pin",
			args:           []string{"pkg:npm/lodash@4.17.21"},
			wantEco:        "npm",
			wantPkg:        "lodash",
			wantPinVersion: "4.17.21",
		},
		{
			name:           "purl pypi pin",
			args:           []string{"pkg:pypi/Django@4.2"},
			wantEco:        "pypi",
			wantPkg:        "django",
			wantPinVersion: "4.2",
		},
		{
			name:    "purl with @version and extra range arg",
			args:    []string{"pkg:npm/lodash@4.17.21", "^4.17.0"},
			wantErr: "version pin and range both given",
		},
		{
			name:    "purl no version no range arg",
			args:    []string{"pkg:npm/lodash"},
			wantErr: "purl has no @version",
		},
		{
			name:    "vers: in first position",
			args:    []string{"vers:npm/lodash/>=1.0"},
			wantErr: "vers: range syntax not yet supported",
		},
		{
			name:    "vers: in second position",
			args:    []string{"pkg:npm/lodash", "vers:npm/>=1.0"},
			wantErr: "vers: range syntax not yet supported",
		},
		{
			name:    "vers: in second legacy position",
			args:    []string{"npm", "lodash", "vers:npm/>=1.0"},
			wantErr: "vers: range syntax not yet supported",
		},
		{
			name:    "unsupported purl type maven",
			args:    []string{"pkg:maven/org.apache.commons/commons-lang3@3.0"},
			wantErr: "unsupported purl type",
		},
		{
			name:    "malformed purl missing prefix",
			args:    []string{"npm:lodash@4.17.21"},
			wantErr: `expected 3 arguments, got 1`,
		},
		{
			name:    "zero args",
			args:    []string{},
			wantErr: "expected arguments, got 0",
		},
		{
			name:    "too many args",
			args:    []string{"pkg:npm/lodash", "^4", "extra"},
			wantErr: "too many arguments",
		},
		{
			name:    "2 args non-purl",
			args:    []string{"npm", "lodash"},
			wantErr: "expected 3 arguments, got 2",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := parseRequest(tc.args)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantEco != "" && req.ecoName != tc.wantEco {
				t.Errorf("ecoName: got %q, want %q", req.ecoName, tc.wantEco)
			}
			if tc.wantPkg != "" && req.pkgName != tc.wantPkg {
				t.Errorf("pkgName: got %q, want %q", req.pkgName, tc.wantPkg)
			}
			if tc.wantRangeExpr != "" && req.rangeExpr != tc.wantRangeExpr {
				t.Errorf("rangeExpr: got %q, want %q", req.rangeExpr, tc.wantRangeExpr)
			}
			if tc.wantPinVersion != "" && req.pinVersion != tc.wantPinVersion {
				t.Errorf("pinVersion: got %q, want %q", req.pinVersion, tc.wantPinVersion)
			}
		})
	}
}

// ---- runWithDeps tests ----

func newFakeSetup() (map[string]ecosystem.Ecosystem, *fakeEco) {
	reg := &fakeRegistry{versions: []ecosystem.Version{
		fakeVersion{"1.0.0"},
		fakeVersion{"2.0.0"},
		fakeVersion{"3.0.0"},
	}}
	eco := &fakeEco{
		name:         "npm",
		registry:     reg,
		rangeAllowed: []string{"1.0.0", "2.0.0"},
	}
	return map[string]ecosystem.Ecosystem{"npm": eco}, eco
}

func TestRunWithDeps_RangeMode(t *testing.T) {
	m, _ := newFakeSetup()
	var stdout, stderr bytes.Buffer
	err := runWithDeps([]string{"npm", "lodash", "^2"}, &stdout, &stderr, fakeLookup(m))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), got)
	}
	if lines[0] != "1.0.0" || lines[1] != "2.0.0" {
		t.Errorf("unexpected output: %v", lines)
	}
}

func TestRunWithDeps_PurlRangeMode(t *testing.T) {
	m, _ := newFakeSetup()
	var stdout, stderr bytes.Buffer
	err := runWithDeps([]string{"pkg:npm/lodash", "^2"}, &stdout, &stderr, fakeLookup(m))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), got)
	}
}

func TestRunWithDeps_PinMode(t *testing.T) {
	m, _ := newFakeSetup()
	var stdout, stderr bytes.Buffer
	err := runWithDeps([]string{"pkg:npm/lodash@2.0.0"}, &stdout, &stderr, fakeLookup(m))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	if got != "2.0.0" {
		t.Errorf("pin mode output: got %q, want %q", got, "2.0.0")
	}
}

func TestRunWithDeps_PinMode_NotFound(t *testing.T) {
	m, _ := newFakeSetup()
	var stdout, stderr bytes.Buffer
	err := runWithDeps([]string{"pkg:npm/lodash@9.9.9"}, &stdout, &stderr, fakeLookup(m))
	if err == nil {
		t.Fatal("expected error for unknown version, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error %q should mention 'not found'", err.Error())
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout, got %q", stdout.String())
	}
}

func TestRunWithDeps_UnknownEcosystem(t *testing.T) {
	m, _ := newFakeSetup()
	var stdout, stderr bytes.Buffer
	err := runWithDeps([]string{"pkg:pypi/requests@2.30"}, &stdout, &stderr, fakeLookup(m))
	if err == nil {
		t.Fatal("expected error for unknown ecosystem, got nil")
	}
	if !strings.Contains(err.Error(), "unknown ecosystem") {
		t.Errorf("error %q should mention 'unknown ecosystem'", err.Error())
	}
}

func TestRunWithDeps_VersRejected(t *testing.T) {
	m, _ := newFakeSetup()
	var stdout, stderr bytes.Buffer
	err := runWithDeps([]string{"vers:npm/lodash/>=1.0"}, &stdout, &stderr, fakeLookup(m))
	if err == nil {
		t.Fatal("expected error for vers: prefix, got nil")
	}
	if !strings.Contains(err.Error(), "vers: range syntax not yet supported") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunWithDeps_VersionFlag(t *testing.T) {
	m, _ := newFakeSetup()
	for _, flag := range []string{"-v", "--version"} {
		var stdout, stderr bytes.Buffer
		err := runWithDeps([]string{flag}, &stdout, &stderr, fakeLookup(m))
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", flag, err)
		}
		if !strings.Contains(stdout.String(), Version) {
			t.Errorf("%s: stdout %q does not contain version %q", flag, stdout.String(), Version)
		}
	}
}

// CLI translates ErrVersionNotFound into a user-facing message and does NOT wrap it.
func TestRunWithDeps_PinMode_VersionNotFound(t *testing.T) {
	m, _ := newFakeSetup()
	var stdout, stderr bytes.Buffer
	err := runWithDeps([]string{"pkg:npm/lodash@9.9.9"}, &stdout, &stderr, fakeLookup(m))
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, resolve.ErrVersionNotFound) {
		t.Fatal("runWithDeps must not wrap ErrVersionNotFound; got unwrappable sentinel")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error %q should contain user-facing 'not found' message", err.Error())
	}
}
