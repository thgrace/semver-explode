package npm

import (
	"fmt"
	"strings"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
)

type op int

const (
	opAny op = iota
	opEQ
	opNE
	opLT
	opLE
	opGT
	opGE
)

type comparator struct {
	op  op
	ver Version
}

type Range struct {
	sets [][]comparator
	raw  string
}

var _ ecosystem.Range = Range{}

// ParseRange parses a node-semver range expression. Supported forms include
// exact versions, comparator operators (>, >=, <, <=, =, !=), partial versions,
// X-ranges, tilde and caret ranges, hyphen ranges, and OR-combined branches.
func ParseRange(s string) (Range, error) {
	raw := s
	branches := splitOr(s)
	sets := make([][]comparator, 0, len(branches))
	for _, br := range branches {
		comps, err := parseBranch(br)
		if err != nil {
			return Range{}, err
		}
		sets = append(sets, comps)
	}
	if len(sets) == 0 {
		sets = append(sets, []comparator{{op: opAny}})
	}
	return Range{sets: sets, raw: raw}, nil
}

func splitOr(s string) []string {
	parts := strings.Split(s, "||")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

func parseBranch(br string) ([]comparator, error) {
	br = strings.TrimSpace(br)
	if br == "" {
		return []comparator{{op: opAny}}, nil
	}

	if expanded, ok, err := tryHyphenRange(br); err != nil {
		return nil, err
	} else if ok {
		return expanded, nil
	}

	tokens, err := tokenize(br)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return []comparator{{op: opAny}}, nil
	}

	comps := make([]comparator, 0, len(tokens))
	for _, tok := range tokens {
		cs, err := parseComparator(tok)
		if err != nil {
			return nil, err
		}
		comps = append(comps, cs...)
	}
	return comps, nil
}

// tryHyphenRange detects "A - B" with surrounding whitespace and expands it.
// It returns ok=true only when the dash sits between whitespace, distinguishing
// it from a prerelease delimiter inside a version.
func tryHyphenRange(br string) ([]comparator, bool, error) {
	fields := strings.Fields(br)
	dashIdx := -1
	for i, f := range fields {
		if f == "-" {
			dashIdx = i
			break
		}
	}
	if dashIdx == -1 {
		return nil, false, nil
	}
	if dashIdx == 0 {
		return nil, false, fmt.Errorf("npm: stray '-' in range %q", br)
	}
	if dashIdx >= len(fields)-1 {
		return nil, false, fmt.Errorf("npm: hyphen range missing upper bound in %q", br)
	}
	left := strings.Join(fields[:dashIdx], " ")
	right := strings.Join(fields[dashIdx+1:], " ")
	// Hyphen ranges should be exactly "A - B"; reject extra comparators.
	if dashIdx != 1 || len(fields)-dashIdx-1 != 1 {
		return nil, false, fmt.Errorf("npm: malformed hyphen range %q", br)
	}
	lo, _, err := parsePartial(left)
	if err != nil {
		return nil, false, fmt.Errorf("npm: bad hyphen range lower bound %q: %w", left, err)
	}
	hi, hiParts, err := parsePartial(right)
	if err != nil {
		return nil, false, fmt.Errorf("npm: bad hyphen range upper bound %q: %w", right, err)
	}
	lower := Version{Major: lo.Major, Minor: lo.Minor, Patch: lo.Patch, Prerelease: lo.Prerelease}
	out := []comparator{{op: opGE, ver: lower}}
	switch hiParts {
	case 3:
		out = append(out, comparator{op: opLE, ver: hi})
	case 2:
		out = append(out, comparator{op: opLT, ver: Version{Major: hi.Major, Minor: hi.Minor + 1, Patch: 0}})
	case 1:
		out = append(out, comparator{op: opLT, ver: Version{Major: hi.Major + 1, Minor: 0, Patch: 0}})
	default:
		return nil, false, fmt.Errorf("npm: hyphen range upper bound has no version")
	}
	return out, true, nil
}

// tokenize splits a branch on whitespace, but re-glues a bare operator
// (>, >=, <, <=, =, !=) onto the following token.
func tokenize(br string) ([]string, error) {
	fields := strings.Fields(br)
	out := make([]string, 0, len(fields))
	i := 0
	for i < len(fields) {
		f := fields[i]
		if isBareOp(f) {
			if i+1 >= len(fields) {
				return nil, fmt.Errorf("npm: operator %q with no operand", f)
			}
			out = append(out, f+fields[i+1])
			i += 2
			continue
		}
		out = append(out, f)
		i++
	}
	return out, nil
}

func isBareOp(s string) bool {
	switch s {
	case ">", ">=", "<", "<=", "=", "!=":
		return true
	}
	return false
}

func parseComparator(tok string) ([]comparator, error) {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return []comparator{{op: opAny}}, nil
	}
	// Wildcard-only tokens.
	if tok == "*" || tok == "x" || tok == "X" {
		return []comparator{{op: opAny}}, nil
	}

	switch tok[0] {
	case '^':
		return parseCaret(tok[1:])
	case '~':
		// Tolerate "~>" as plain "~" (some ecosystems write it; node-semver
		// accepts the bare tilde — we accept the "~" prefix only).
		return parseTilde(strings.TrimPrefix(tok[1:], ">"))
	}

	op, rest := splitOp(tok)
	if rest == "" {
		return nil, fmt.Errorf("npm: comparator %q has no version", tok)
	}
	// Guard against doubled operators like ">>1" or ">=>2".
	if len(rest) > 0 {
		switch rest[0] {
		case '>', '<', '=', '!':
			return nil, fmt.Errorf("npm: malformed comparator %q", tok)
		}
	}

	pv, parts, err := parsePartial(rest)
	if err != nil {
		return nil, fmt.Errorf("npm: bad comparator %q: %w", tok, err)
	}

	if parts < 3 || isWildcardPartial(rest) {
		return expandPartial(op, pv, parts, rest)
	}

	return []comparator{{op: op, ver: pv}}, nil
}

func splitOp(tok string) (op, string) {
	switch {
	case strings.HasPrefix(tok, ">="):
		return opGE, strings.TrimSpace(tok[2:])
	case strings.HasPrefix(tok, "<="):
		return opLE, strings.TrimSpace(tok[2:])
	case strings.HasPrefix(tok, "!="):
		return opNE, strings.TrimSpace(tok[2:])
	case strings.HasPrefix(tok, ">"):
		return opGT, strings.TrimSpace(tok[1:])
	case strings.HasPrefix(tok, "<"):
		return opLT, strings.TrimSpace(tok[1:])
	case strings.HasPrefix(tok, "="):
		return opEQ, strings.TrimSpace(tok[1:])
	}
	return opEQ, tok
}

func isWildcardPartial(s string) bool {
	s = stripVPrefix(s)
	parts := strings.Split(stripPreBuild(s), ".")
	for _, p := range parts {
		if isWildcardComponent(p) || p == "" {
			return true
		}
	}
	return false
}

func isWildcardComponent(p string) bool {
	return p == "x" || p == "X" || p == "*"
}

func stripPreBuild(s string) string {
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		return s[:i]
	}
	return s
}

func stripVPrefix(s string) string {
	if len(s) > 0 && (s[0] == 'v' || s[0] == 'V') {
		return s[1:]
	}
	return s
}

// parsePartial parses a possibly-partial version (e.g. "1", "1.2", "1.2.x",
// "1.2.3-beta"). It returns the version with absent components filled with 0,
// and the count of *concrete* leading components (a wildcard counts as absent
// from that point onward).
func parsePartial(s string) (Version, int, error) {
	s = strings.TrimSpace(s)
	s = stripVPrefix(s)
	if s == "" {
		return Version{}, 0, fmt.Errorf("empty version")
	}
	if isWildcardComponent(s) {
		return Version{}, 0, nil
	}

	core := s
	var preStr, buildStr string
	if i := strings.IndexByte(core, '+'); i >= 0 {
		buildStr = core[i+1:]
		core = core[:i]
	}
	if i := strings.IndexByte(core, '-'); i >= 0 {
		preStr = core[i+1:]
		core = core[:i]
	}

	parts := strings.Split(core, ".")
	if len(parts) > 3 {
		return Version{}, 0, fmt.Errorf("too many components in %q", s)
	}

	var v Version
	concrete := 0
	wildSeen := false
	nums := []*uint64{&v.Major, &v.Minor, &v.Patch}
	for i, p := range parts {
		if p == "" || isWildcardComponent(p) {
			wildSeen = true
			continue
		}
		if wildSeen {
			return Version{}, 0, fmt.Errorf("component after wildcard in %q", s)
		}
		n, err := parseNumeric(p)
		if err != nil {
			return Version{}, 0, fmt.Errorf("bad component %q: %w", p, err)
		}
		*nums[i] = n
		concrete = i + 1
	}

	if preStr != "" {
		if concrete != 3 {
			return Version{}, 0, fmt.Errorf("prerelease on partial version %q", s)
		}
		pr, err := parseDotIdents(preStr, true)
		if err != nil {
			return Version{}, 0, fmt.Errorf("bad prerelease: %w", err)
		}
		v.Prerelease = pr
	}
	if buildStr != "" {
		b, err := parseDotIdents(buildStr, false)
		if err != nil {
			return Version{}, 0, fmt.Errorf("bad build: %w", err)
		}
		v.Build = b
	}
	return v, concrete, nil
}

// expandPartial turns a partial/x-range like "1.2" with an optional operator
// into one or two concrete comparators per node-semver semantics.
func expandPartial(op op, pv Version, parts int, raw string) ([]comparator, error) {
	switch op {
	case opEQ:
		return expandEqPartial(pv, parts)
	case opGT:
		switch parts {
		case 0:
			// ">*" — impossible; matches nothing. Use <0.0.0 trick.
			return []comparator{{op: opLT, ver: Version{}}}, nil
		case 1:
			return []comparator{{op: opGE, ver: Version{Major: pv.Major + 1}}}, nil
		case 2:
			return []comparator{{op: opGE, ver: Version{Major: pv.Major, Minor: pv.Minor + 1}}}, nil
		case 3:
			return []comparator{{op: opGT, ver: pv}}, nil
		}
	case opGE:
		switch parts {
		case 0:
			return []comparator{{op: opAny}}, nil
		case 1, 2, 3:
			return []comparator{{op: opGE, ver: pv}}, nil
		}
	case opLT:
		switch parts {
		case 0:
			return []comparator{{op: opLT, ver: Version{}}}, nil
		case 1, 2, 3:
			return []comparator{{op: opLT, ver: pv}}, nil
		}
	case opLE:
		switch parts {
		case 0:
			return []comparator{{op: opAny}}, nil
		case 1:
			return []comparator{{op: opLT, ver: Version{Major: pv.Major + 1}}}, nil
		case 2:
			return []comparator{{op: opLT, ver: Version{Major: pv.Major, Minor: pv.Minor + 1}}}, nil
		case 3:
			return []comparator{{op: opLE, ver: pv}}, nil
		}
	case opNE:
		if parts == 3 {
			return []comparator{{op: opNE, ver: pv}}, nil
		}
		return nil, fmt.Errorf("npm: != requires a full version, got %q", raw)
	}
	return nil, fmt.Errorf("npm: cannot expand partial %q", raw)
}

func expandEqPartial(pv Version, parts int) ([]comparator, error) {
	switch parts {
	case 0:
		return []comparator{{op: opAny}}, nil
	case 1:
		return []comparator{
			{op: opGE, ver: Version{Major: pv.Major}},
			{op: opLT, ver: Version{Major: pv.Major + 1}},
		}, nil
	case 2:
		return []comparator{
			{op: opGE, ver: Version{Major: pv.Major, Minor: pv.Minor}},
			{op: opLT, ver: Version{Major: pv.Major, Minor: pv.Minor + 1}},
		}, nil
	case 3:
		return []comparator{{op: opEQ, ver: pv}}, nil
	}
	return nil, fmt.Errorf("npm: unreachable")
}

func parseTilde(rest string) ([]comparator, error) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return nil, fmt.Errorf("npm: ~ with no version")
	}
	pv, parts, err := parsePartial(rest)
	if err != nil {
		return nil, fmt.Errorf("npm: bad tilde %q: %w", rest, err)
	}
	switch parts {
	case 0:
		return []comparator{{op: opAny}}, nil
	case 1:
		return []comparator{
			{op: opGE, ver: Version{Major: pv.Major}},
			{op: opLT, ver: Version{Major: pv.Major + 1}},
		}, nil
	case 2, 3:
		lower := Version{Major: pv.Major, Minor: pv.Minor, Patch: pv.Patch, Prerelease: pv.Prerelease}
		return []comparator{
			{op: opGE, ver: lower},
			{op: opLT, ver: Version{Major: pv.Major, Minor: pv.Minor + 1}},
		}, nil
	}
	return nil, fmt.Errorf("npm: unreachable")
}

func parseCaret(rest string) ([]comparator, error) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return nil, fmt.Errorf("npm: ^ with no version")
	}
	pv, parts, err := parsePartial(rest)
	if err != nil {
		return nil, fmt.Errorf("npm: bad caret %q: %w", rest, err)
	}

	if parts == 0 {
		return []comparator{{op: opAny}}, nil
	}

	lower := Version{Major: pv.Major, Minor: pv.Minor, Patch: pv.Patch, Prerelease: pv.Prerelease}
	var upper Version
	switch {
	case pv.Major > 0 || parts == 1:
		upper = Version{Major: pv.Major + 1}
	case pv.Minor > 0 || parts == 2:
		upper = Version{Major: 0, Minor: pv.Minor + 1}
	default:
		// pv.Major == 0, pv.Minor == 0, parts == 3.
		upper = Version{Major: 0, Minor: 0, Patch: pv.Patch + 1}
	}
	return []comparator{
		{op: opGE, ver: lower},
		{op: opLT, ver: upper},
	}, nil
}

// Contains reports whether v satisfies the range. The npm prerelease gate
// requires that a prerelease v only match a branch whose comparators
// explicitly mention a prerelease at the same major.minor.patch — this
// matches canonical node-semver behavior, not the univers reference.
func (r Range) Contains(v ecosystem.Version) bool {
	nv, ok := v.(Version)
	if !ok {
		return false
	}
	for _, set := range r.sets {
		if branchMatches(set, nv) {
			return true
		}
	}
	return false
}

func branchMatches(set []comparator, v Version) bool {
	if v.IsPrerelease() {
		if !prereleaseGateAllows(set, v) {
			return false
		}
	}
	for _, c := range set {
		if !compMatches(c, v) {
			return false
		}
	}
	return true
}

func prereleaseGateAllows(set []comparator, v Version) bool {
	for _, c := range set {
		if c.op == opAny {
			continue
		}
		if !c.ver.IsPrerelease() {
			continue
		}
		if c.ver.Major == v.Major && c.ver.Minor == v.Minor && c.ver.Patch == v.Patch {
			return true
		}
	}
	return false
}

func compMatches(c comparator, v Version) bool {
	switch c.op {
	case opAny:
		return true
	case opEQ:
		return v.cmp(c.ver) == 0
	case opNE:
		return v.cmp(c.ver) != 0
	case opLT:
		return v.cmp(c.ver) < 0
	case opLE:
		return v.cmp(c.ver) <= 0
	case opGT:
		return v.cmp(c.ver) > 0
	case opGE:
		return v.cmp(c.ver) >= 0
	}
	return false
}

func (r Range) String() string {
	return r.raw
}
