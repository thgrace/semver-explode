package pypi

import (
	"fmt"
	"strings"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
)

type op = ecosystem.Op

const (
	opEQ = ecosystem.OpEQ // == (exact, with local-stripping rule)
	opNE = ecosystem.OpNE // !=
	opLT = ecosystem.OpLT // <
	opLE = ecosystem.OpLE // <=
	opGT = ecosystem.OpGT // >
	opGE = ecosystem.OpGE // >=
)

// pypi-specific ops, numbered outside the shared range.
const (
	opCom op = 100 + iota // ~= compatible release
	opArb                 // === arbitrary string equality
	opEQW                 // ==X.* prefix wildcard
	opNEW                 // !=X.* negated prefix wildcard
)

type comparator struct {
	op     op
	ver    Version
	prefix versionPrefix // epoch + release prefix for opEQW / opNEW
	rawVer string        // for opArb (case-folded raw string)
}

type versionPrefix struct {
	epoch   int
	release []int
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

	// Arbitrary equality (===) does raw, case-folded string comparison.
	// We do NOT require the RHS to be a valid PEP 440 version, so unparseable
	// strings like "===foobar" are accepted. We still probe it as a Version so
	// that "===1.0a1" can opt the set into prerelease candidates (mirroring
	// packaging.specifiers.Specifier.prereleases for the === operator); the
	// probe failure is non-fatal and contributes a zero Version.
	if o == opArb {
		return comparator{op: opArb, rawVer: asciiFold(rest), ver: prereleaseProbe(rest)}, nil
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
		if len(v.Local) > 0 {
			return comparator{}, fmt.Errorf("pypi: local versions are not permitted in %q", s)
		}
		return comparator{op: opCom, ver: v}, nil
	}

	v, err := ParseVersion(rest)
	if err != nil {
		return comparator{}, fmt.Errorf("pypi: bad version in %q: %w", s, err)
	}
	if (o == opLT || o == opLE || o == opGT || o == opGE) && len(v.Local) > 0 {
		return comparator{}, fmt.Errorf("pypi: local versions are not permitted in %q", s)
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

// parseWildcardPrefix parses the release part before ".*" (for example
// "1!1.4" becomes epoch 1 and release prefix [1,4]).
func parseWildcardPrefix(s string) (versionPrefix, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return versionPrefix{}, fmt.Errorf("empty prefix")
	}
	v, err := ParseVersion(s)
	if err != nil {
		return versionPrefix{}, err
	}
	if v.Pre != nil || v.Post != nil || v.Dev != nil || len(v.Local) > 0 {
		return versionPrefix{}, fmt.Errorf("prefix wildcard must use only epoch and release segments")
	}
	return versionPrefix{epoch: v.Epoch, release: v.Release}, nil
}

// prereleaseProbe tries to parse s as a Version; on failure returns a zero
// Version. Used only to determine whether a === spec opts the set into
// prerelease candidates; it never gates the actual === string match.
func prereleaseProbe(s string) Version {
	v, err := ParseVersion(s)
	if err != nil {
		return Version{}
	}
	return v
}

func asciiFold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if 'A' <= ch && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		b.WriteByte(ch)
	}
	return b.String()
}

// setAllowsPrereleases returns true when any specifier explicitly names a
// prerelease. Prefix wildcards (==X.*) do not opt in by themselves because
// their stored prefix is always a plain release tuple. For === we consult
// the parse-probe result so "===1.0a1" can opt the set into prereleases
// (mirroring packaging.specifiers.Specifier.prereleases).
func setAllowsPrereleases(comps []comparator) bool {
	for _, c := range comps {
		switch c.op {
		case opEQ, opLT, opLE, opGT, opGE, opCom, opArb:
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
		return r.containsArbitrary(v)
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

func (r Range) containsArbitrary(v ecosystem.Version) bool {
	if len(r.comps) == 0 {
		return false
	}
	if v.IsPrerelease() && !setAllowsPrereleases(r.comps) {
		return false
	}
	for _, c := range r.comps {
		if c.op != opArb || asciiFold(v.String()) != c.rawVer {
			return false
		}
	}
	return true
}

// compMatches does NOT delegate to ecosystem.MatchOrdered for the six shared
// ops, because PEP 440 assigns non-trivial semantics to four of them:
//   - opEQ / opNE strip the candidate's local version when the constraint has none
//   - opLT / opGT exclude any version sharing the constraint's release tuple
//
// Only opLE and opGE use plain sign-based comparison. We share the Op vocabulary
// (via ecosystem.Op constants) but not the match logic.
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
		return asciiFold(v.String()) == c.rawVer
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

// ltMatches implements < exclusive bound, mirroring packaging.specifiers
// _compare_less_than: candidate < constraint, and if the constraint itself is
// not a prerelease, also exclude prereleases that share its release tuple
// (so "<1.0" excludes "1.0a1"). When the constraint IS a prerelease, no such
// exclusion applies: "<1.0a2" matches "1.0a1".
func ltMatches(constraint, candidate Version) bool {
	if candidate.cmp(constraint) >= 0 {
		return false
	}
	if !constraint.IsPrerelease() && candidate.IsPrerelease() {
		if sameRelease(constraint, candidate) {
			return false
		}
	}
	return true
}

// gtMatches implements > exclusive bound, mirroring packaging.specifiers
// _compare_greater_than: candidate > constraint, plus two same-release
// exclusions when the constraint is a plain release:
//   - postreleases of the same release ("1.0.post1" is not > "1.0")
//   - local versions of the same release ("1.0+local" is not > "1.0")
//
// When the constraint itself is a postrelease ("1.0.post1"), postreleases of
// the same release ARE allowed as long as they compare greater.
func gtMatches(constraint, candidate Version) bool {
	if candidate.cmp(constraint) <= 0 {
		return false
	}
	if constraint.Post == nil && candidate.Post != nil {
		if sameRelease(constraint, candidate) {
			return false
		}
	}
	if len(candidate.Local) > 0 {
		if sameRelease(constraint, candidate) {
			return false
		}
	}
	return true
}

// sameRelease returns true when a and b share the same epoch and release tuple.
func sameRelease(a, b Version) bool {
	if a.Epoch != b.Epoch {
		return false
	}
	return cmpRelease(a.Release, b.Release) == 0
}

// comMatches implements ~= (compatible release): ~=X.Y is equivalent to
// >=X.Y, ==X.*; ~=X.Y.Z is equivalent to >=X.Y.Z, ==X.Y.*. The prefix is all
// release segments except the last, preserving trailing zeros to control width.
func comMatches(constraint, candidate Version) bool {
	if candidate.cmp(constraint) < 0 {
		return false
	}
	prefix := versionPrefix{
		epoch:   constraint.Epoch,
		release: constraint.Release[:len(constraint.Release)-1],
	}
	return prefixMatches(prefix, candidate)
}

// prefixMatches checks that the candidate has the same epoch and release
// prefix. Pre/post/dev/local are ignored; candidate release segments are
// zero-padded as needed, but the prefix width is preserved.
func prefixMatches(prefix versionPrefix, candidate Version) bool {
	if candidate.Epoch != prefix.epoch {
		return false
	}
	for i, p := range prefix.release {
		got := 0
		if i < len(candidate.Release) {
			got = candidate.Release[i]
		}
		if got != p {
			return false
		}
	}
	return true
}

func (r Range) String() string {
	return r.raw
}
