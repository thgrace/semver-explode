package npm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
)

// Version is a node-semver compatible version per semver 2.0.0.
// Build metadata is preserved on the value but ignored in Compare.
type Version struct {
	Major      uint64
	Minor      uint64
	Patch      uint64
	Prerelease []string
	Build      []string
}

var _ ecosystem.Version = Version{}

// ParseVersion parses s as a strict semver 2.0.0 version, tolerating a single
// leading 'v' or 'V' and surrounding whitespace (matching node-semver's loose
// parsing for that one prefix).
func ParseVersion(s string) (Version, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return Version{}, fmt.Errorf("npm: empty version")
	}
	if raw[0] == 'v' || raw[0] == 'V' {
		raw = raw[1:]
	}

	core, pre, build, err := splitVersion(raw)
	if err != nil {
		return Version{}, err
	}

	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("npm: version %q must have 3 numeric components", s)
	}

	maj, err := parseNumeric(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("npm: bad major in %q: %w", s, err)
	}
	min, err := parseNumeric(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("npm: bad minor in %q: %w", s, err)
	}
	pat, err := parseNumeric(parts[2])
	if err != nil {
		return Version{}, fmt.Errorf("npm: bad patch in %q: %w", s, err)
	}

	v := Version{Major: maj, Minor: min, Patch: pat}
	if pre != "" {
		v.Prerelease, err = parseDotIdents(pre, true)
		if err != nil {
			return Version{}, fmt.Errorf("npm: bad prerelease in %q: %w", s, err)
		}
	}
	if build != "" {
		v.Build, err = parseDotIdents(build, false)
		if err != nil {
			return Version{}, fmt.Errorf("npm: bad build in %q: %w", s, err)
		}
	}
	return v, nil
}

// splitVersion separates "1.2.3-pre+build" into ("1.2.3", "pre", "build").
func splitVersion(raw string) (core, pre, build string, err error) {
	core = raw
	if i := strings.IndexByte(raw, '+'); i >= 0 {
		core = raw[:i]
		build = raw[i+1:]
	}
	if i := strings.IndexByte(core, '-'); i >= 0 {
		pre = core[i+1:]
		core = core[:i]
	}
	if core == "" {
		return "", "", "", fmt.Errorf("npm: missing version core in %q", raw)
	}
	return core, pre, build, nil
}

func parseNumeric(s string) (uint64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty numeric component")
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, fmt.Errorf("leading zero in %q", s)
	}
	return strconv.ParseUint(s, 10, 64)
}

func parseDotIdents(s string, isPrerelease bool) ([]string, error) {
	parts := strings.Split(s, ".")
	for _, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("empty identifier")
		}
		if !isIdent(p) {
			return nil, fmt.Errorf("invalid identifier %q", p)
		}
		if isPrerelease && isAllDigits(p) && len(p) > 1 && p[0] == '0' {
			return nil, fmt.Errorf("numeric prerelease identifier %q has leading zero", p)
		}
	}
	return parts, nil
}

func isIdent(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && c != '-' {
			return false
		}
	}
	return len(s) > 0
}

func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

// String returns the canonical "MAJOR.MINOR.PATCH[-PRE][+BUILD]" form.
func (v Version) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d.%d.%d", v.Major, v.Minor, v.Patch)
	if len(v.Prerelease) > 0 {
		b.WriteByte('-')
		b.WriteString(strings.Join(v.Prerelease, "."))
	}
	if len(v.Build) > 0 {
		b.WriteByte('+')
		b.WriteString(strings.Join(v.Build, "."))
	}
	return b.String()
}

// IsPrerelease reports whether the version carries any prerelease identifier.
func (v Version) IsPrerelease() bool {
	return len(v.Prerelease) > 0
}

// Compare returns -1/0/+1 per semver 2.0 precedence rules. Build metadata is
// ignored. The argument must be a npm.Version, otherwise Compare panics — the
// ecosystem.Version interface is intentionally per-ecosystem.
func (v Version) Compare(other ecosystem.Version) int {
	o, ok := other.(Version)
	if !ok {
		panic(fmt.Sprintf("npm: cannot compare npm.Version with %T", other))
	}
	return v.cmp(o)
}

func (v Version) cmp(o Version) int {
	if c := cmpUint(v.Major, o.Major); c != 0 {
		return c
	}
	if c := cmpUint(v.Minor, o.Minor); c != 0 {
		return c
	}
	if c := cmpUint(v.Patch, o.Patch); c != 0 {
		return c
	}
	return comparePrerelease(v.Prerelease, o.Prerelease)
}

func cmpUint(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// comparePrerelease implements semver 2.0 §11: a version without a prerelease
// has higher precedence than one with; identifiers are compared left-to-right
// (numeric < alphanumeric; numerics by value; alphanumerics by ASCII), and a
// longer prerelease wins when the shared prefix is equal.
func comparePrerelease(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if c := cmpIdent(a[i], b[i]); c != 0 {
			return c
		}
	}
	return cmpInt(len(a), len(b))
}

func cmpIdent(a, b string) int {
	aNum, aIsNum := identNumeric(a)
	bNum, bIsNum := identNumeric(b)
	switch {
	case aIsNum && bIsNum:
		return cmpUint(aNum, bNum)
	case aIsNum:
		return -1
	case bIsNum:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

func identNumeric(s string) (uint64, bool) {
	if !isAllDigits(s) {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
