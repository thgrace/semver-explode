package pypi

import (
	"fmt"
	"strings"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
)

type op int

const (
	opEQ  op = iota // == (exact, with local-stripping rule)
	opNE            // !=
	opLT            // <
	opLE            // <=
	opGT            // >
	opGE            // >=
	opCom           // ~= compatible release
	opArb           // === arbitrary string equality
	opEQW           // ==X.* prefix wildcard
	opNEW           // !=X.* negated prefix wildcard
)

type comparator struct {
	op     op
	ver    Version
	prefix []int  // release prefix for opEQW / opNEW
	rawVer string // for opArb (normalized String())
}

// Range holds a parsed PEP 440 specifier set; AND semantics across all specifiers.
type Range struct {
	comps []comparator
	raw   string
}

var _ ecosystem.Range = Range{}

// ParseRange parses a PEP 440 specifier set string (comma-separated specifiers).
func ParseRange(s string) (Range, error) {
	raw := s
	s = strings.TrimSpace(s)
	if s == "" {
		return Range{raw: raw}, nil
	}
	parts := strings.Split(s, ",")
	comps := make([]comparator, 0, len(parts))
	for _, p := range parts {
		c, err := parseSpecifier(strings.TrimSpace(p))
		if err != nil {
			return Range{}, err
		}
		comps = append(comps, c)
	}
	return Range{comps: comps, raw: raw}, nil
}

func parseSpecifier(s string) (comparator, error) {
	if s == "" {
		return comparator{}, fmt.Errorf("pypi: empty specifier")
	}

	o, rest, err := splitOp(s)
	if err != nil {
		return comparator{}, err
	}
	if rest == "" {
		return comparator{}, fmt.Errorf("pypi: specifier %q has no version", s)
	}

	// Wildcard suffix only valid for == and !=
	if strings.HasSuffix(rest, ".*") {
		switch o {
		case opEQ, opNE:
			prefix, err := parseWildcardPrefix(rest[:len(rest)-2])
			if err != nil {
				return comparator{}, fmt.Errorf("pypi: bad wildcard specifier %q: %w", s, err)
			}
			wo := opEQW
			if o == opNE {
				wo = opNEW
			}
			return comparator{op: wo, prefix: prefix}, nil
		default:
			return comparator{}, fmt.Errorf("pypi: .* wildcard not valid with this operator in %q", s)
		}
	}

	// Arbitrary string equality: normalize and store.
	if o == opArb {
		v, err := ParseVersion(rest)
		if err != nil {
			return comparator{}, fmt.Errorf("pypi: bad version in %q: %w", s, err)
		}
		return comparator{op: opArb, rawVer: v.String()}, nil
	}

	// ~= requires at least two release segments.
	if o == opCom {
		v, err := ParseVersion(rest)
		if err != nil {
			return comparator{}, fmt.Errorf("pypi: bad version in %q: %w", s, err)
		}
		if len(v.Release) < 2 {
			return comparator{}, fmt.Errorf("pypi: ~= requires at least two release segments in %q", s)
		}
		return comparator{op: opCom, ver: v}, nil
	}

	v, err := ParseVersion(rest)
	if err != nil {
		return comparator{}, fmt.Errorf("pypi: bad version in %q: %w", s, err)
	}
	return comparator{op: o, ver: v}, nil
}

func splitOp(s string) (op, string, error) {
	switch {
	case strings.HasPrefix(s, "==="):
		return opArb, strings.TrimSpace(s[3:]), nil
	case strings.HasPrefix(s, "~="):
		return opCom, strings.TrimSpace(s[2:]), nil
	case strings.HasPrefix(s, "=="):
		return opEQ, strings.TrimSpace(s[2:]), nil
	case strings.HasPrefix(s, "!="):
		return opNE, strings.TrimSpace(s[2:]), nil
	case strings.HasPrefix(s, ">="):
		return opGE, strings.TrimSpace(s[2:]), nil
	case strings.HasPrefix(s, "<="):
		return opLE, strings.TrimSpace(s[2:]), nil
	case strings.HasPrefix(s, ">"):
		return opGT, strings.TrimSpace(s[1:]), nil
	case strings.HasPrefix(s, "<"):
		return opLT, strings.TrimSpace(s[1:]), nil
	}
	return 0, "", fmt.Errorf("pypi: unrecognized operator in specifier %q", s)
}

// parseWildcardPrefix parses the release part before ".*" (e.g. "1.4" → [1,4]).
func parseWildcardPrefix(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty prefix")
	}
	// Allow optional epoch prefix (e.g. "1!1.4.*")
	parts := strings.Split(s, ".")
	prefix := make([]int, 0, len(parts))
	for _, p := range parts {
		var n int
		if _, err := fmt.Sscanf(p, "%d", &n); err != nil {
			return nil, fmt.Errorf("non-numeric segment %q", p)
		}
		prefix = append(prefix, n)
	}
	return prefix, nil
}

// setAllowsPrereleases returns true when any specifier explicitly involves a
// prerelease: either a constraint version that is a prerelease, or a wildcard
// ==X.* / !=X.* specifier (which by PEP 440 matches pre/post/dev releases too).
func setAllowsPrereleases(comps []comparator) bool {
	for _, c := range comps {
		switch c.op {
		case opEQW:
			// ==X.* explicitly includes pre-releases of the prefix
			return true
		case opNEW, opArb:
			continue
		default:
			if c.ver.IsPrerelease() {
				return true
			}
		}
	}
	return false
}

// Contains reports whether v satisfies the specifier set.
// Prerelease filter: if v is a prerelease and no specifier in the set
// names a prerelease version, we reject v regardless of operator matches.
// This matches packaging.specifiers.SpecifierSet default behavior; we do not
// expose an allow_prereleases override (not needed for CLI use).
func (r Range) Contains(v ecosystem.Version) bool {
	pv, ok := v.(Version)
	if !ok {
		return false
	}
	// Empty set matches everything except prereleases (per packaging semantics).
	if pv.IsPrerelease() && !setAllowsPrereleases(r.comps) {
		return false
	}
	for _, c := range r.comps {
		if !compMatches(c, pv) {
			return false
		}
	}
	return true
}

func compMatches(c comparator, v Version) bool {
	switch c.op {
	case opEQ:
		return eqMatches(c.ver, v)
	case opNE:
		return !eqMatches(c.ver, v)
	case opLT:
		return ltMatches(c.ver, v)
	case opLE:
		return v.cmp(c.ver) <= 0
	case opGT:
		return gtMatches(c.ver, v)
	case opGE:
		return v.cmp(c.ver) >= 0
	case opCom:
		return comMatches(c.ver, v)
	case opArb:
		return v.String() == c.rawVer
	case opEQW:
		return prefixMatches(c.prefix, v)
	case opNEW:
		return !prefixMatches(c.prefix, v)
	}
	return false
}

// eqMatches implements PEP 440 == semantics: if the constraint has no local
// version, the candidate's local version is ignored for comparison purposes.
func eqMatches(constraint, candidate Version) bool {
	if len(constraint.Local) == 0 {
		// strip local from candidate for comparison
		stripped := candidate
		stripped.Local = nil
		return stripped.cmp(constraint) == 0
	}
	return candidate.cmp(constraint) == 0
}

// ltMatches implements < exclusive bound: excludes any version whose release
// tuple equals the constraint's, including pre/post/dev of that release.
// Per PEP 440 §5.6, <V excludes V itself and all pre/post/dev of V's release.
func ltMatches(constraint, candidate Version) bool {
	if sameRelease(constraint, candidate) {
		return false
	}
	return candidate.cmp(constraint) < 0
}

// gtMatches implements > exclusive bound: excludes any version whose release
// tuple equals the constraint's release tuple (including pre/post/dev of it).
func gtMatches(constraint, candidate Version) bool {
	if sameRelease(constraint, candidate) {
		return false
	}
	return candidate.cmp(constraint) > 0
}

// sameRelease returns true when a and b share the same epoch and release tuple.
func sameRelease(a, b Version) bool {
	if a.Epoch != b.Epoch {
		return false
	}
	return cmpRelease(a.Release, b.Release) == 0
}

// comMatches implements ~= (compatible release): ~=X.Y ≡ >=X.Y, ==X.*
// ~=X.Y.Z ≡ >=X.Y.Z, ==X.Y.*  (prefix is all segments except the last)
func comMatches(constraint, candidate Version) bool {
	if candidate.cmp(constraint) < 0 {
		return false
	}
	prefix := constraint.Release[:len(constraint.Release)-1]
	return prefixMatches(prefix, candidate)
}

// prefixMatches checks that the candidate's release starts with the given prefix.
// Pre/post/dev/local are ignored — only release segments are compared.
func prefixMatches(prefix []int, candidate Version) bool {
	rel := stripTrailingZeros(candidate.Release)
	pfx := stripTrailingZeros(prefix)
	// prefix must not be longer than the candidate's release
	if len(pfx) > len(rel) {
		// pad rel with zeros up to prefix length
		padded := make([]int, len(pfx))
		copy(padded, rel)
		rel = padded
	}
	for i, p := range pfx {
		if rel[i] != p {
			return false
		}
	}
	return true
}

func (r Range) String() string {
	return r.raw
}
