package purl

import (
	"strings"
	"testing"
)

type successCase struct {
	input   string
	wantEco string
	wantPkg string
	wantVer string
}

var successCases = []successCase{
	{
		input:   "pkg:npm/lodash",
		wantEco: "npm",
		wantPkg: "lodash",
		wantVer: "",
	},
	{
		input:   "pkg://npm/lodash",
		wantEco: "npm",
		wantPkg: "lodash",
		wantVer: "",
	},
	{
		input:   "pkg:npm/%40babel/core@7.0.0",
		wantEco: "npm",
		wantPkg: "@babel/core",
		wantVer: "7.0.0",
	},
	{
		input:   "pkg:pypi/Django@4.2",
		wantEco: "pypi",
		wantPkg: "django",
		wantVer: "4.2",
	},
	{
		input:   "pkg:pypi/django_allauth@1.0",
		wantEco: "pypi",
		wantPkg: "django-allauth",
		wantVer: "1.0",
	},
	{
		input:   "pkg:pypi/my.pkg_name@1.0",
		wantEco: "pypi",
		wantPkg: "my-pkg-name",
		wantVer: "1.0",
	},
	{
		input:   "pkg:npm/React@18.2.0",
		wantEco: "npm",
		wantPkg: "react",
		wantVer: "18.2.0",
	},
	{
		input:   "pkg:npm/%40Babel/Core@7.0.0",
		wantEco: "npm",
		wantPkg: "@babel/core",
		wantVer: "7.0.0",
	},
	{
		input:   "pkg:npm/lodash?vcs_url=git%2Bhttps%3A%2F%2Fgithub.com%2Flodash%2Flodash",
		wantEco: "npm",
		wantPkg: "lodash",
		wantVer: "",
	},
	{
		input:   "pkg:npm/lodash#dist/index.js",
		wantEco: "npm",
		wantPkg: "lodash",
		wantVer: "",
	},
}

func TestParse_QualifierPlusLiteral(t *testing.T) {
	p, err := Parse("pkg:npm/lodash?vcs_url=git+https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := p.Qualifiers["vcs_url"]; got != "git+https://example.com" {
		t.Errorf("vcs_url = %q, want %q", got, "git+https://example.com")
	}
}

func TestParse_Success(t *testing.T) {
	for _, tc := range successCases {
		t.Run(tc.input, func(t *testing.T) {
			p, err := Parse(tc.input)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tc.input, err)
			}

			gotEco := p.Ecosystem()
			if gotEco != tc.wantEco {
				t.Errorf("Ecosystem() = %q, want %q", gotEco, tc.wantEco)
			}

			gotPkg, err := p.PackageName()
			if err != nil {
				t.Fatalf("PackageName() unexpected error: %v", err)
			}
			if gotPkg != tc.wantPkg {
				t.Errorf("PackageName() = %q, want %q", gotPkg, tc.wantPkg)
			}

			if p.Version != tc.wantVer {
				t.Errorf("Version = %q, want %q", p.Version, tc.wantVer)
			}
		})
	}
}

type errorCase struct {
	input       string
	errContains string
}

var errorCases = []errorCase{
	{
		input:       "npm/lodash",
		errContains: "missing \"pkg:\"",
	},
	{
		input:       "pkg:npm/",
		errContains: "empty",
	},
	{
		input:       "pkg:n+pm/lodash",
		errContains: "invalid type",
	},
	{
		input:       "pkg:maven/org.apache.commons/commons-lang3",
		errContains: "unsupported type",
	},
	{
		input:       "pkg:pypi/org/django",
		errContains: "namespace",
	},
	{
		input:       "pkg:npm/myorg/lodash",
		errContains: "scope",
	},
	{
		input:       "pkg:npm/lodash%ZZbad",
		errContains: "malformed percent-encoding",
	},
}

func TestParse_Errors(t *testing.T) {
	for _, tc := range errorCases {
		t.Run(tc.input, func(t *testing.T) {
			p, err := Parse(tc.input)
			if err == nil {
				// Some errors surface in PackageName rather than Parse.
				_, pkgErr := p.PackageName()
				if pkgErr == nil {
					t.Fatalf("Parse(%q) + PackageName() expected error containing %q, got none", tc.input, tc.errContains)
				}
				if !strings.Contains(pkgErr.Error(), tc.errContains) {
					t.Fatalf("PackageName() error = %q, want substring %q", pkgErr.Error(), tc.errContains)
				}
				return
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("Parse(%q) error = %q, want substring %q", tc.input, err.Error(), tc.errContains)
			}
		})
	}
}
