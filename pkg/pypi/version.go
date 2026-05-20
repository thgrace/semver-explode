package pypi

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
)

// Version holds a parsed PEP 440 version.
type Version struct {
	Epoch   int
	Release []int // as written; trailing zeros are preserved for String()
	Pre     *preRelease
	Post    *int // nil = no post
	Dev     *int // nil = no dev
	Local   []localSegment
}

var _ ecosystem.Version = Version{}

type preRelease struct {
	Label string // "a", "b", or "rc"
	N     int
}

// localSegment is one dot-separated piece of a local version label.
// If IsNum is true, Num is the value; otherwise Str holds the text.
type localSegment struct {
	IsNum bool
	Num   int
	Str   string
}

// PEP 440 regex, matching the packaging library's pattern.
// Accepts v/V prefix, optional epoch, release, optional pre/post/dev, optional local.
var pep440Re = regexp.MustCompile(
	`(?i)` +
		`^v?` +
		`(?:(\d+)!)?` + // epoch
		`(\d+(?:\.\d+)*)` + // release (required)
		`(?:[-_.]?` + // optional separator before pre
		`(a|alpha|b|beta|c|rc|pre|preview)` + // pre label
		`[-_.]?(\d*)` + // pre number (optional)
		`)?` +
		`(?:` + // post
		`(?:[-_.]?(post|rev|r)[-_.]?(\d*))` + // .postN / .revN forms
		`|(?:-((\d+)))` + // bare -N form
		`)?` +
		`(?:[-_.]?(dev)[-_.]?(\d*))?` + // dev
		`(?:\+([a-z0-9](?:[a-z0-9._-]*[a-z0-9])?))?` + // local
		`$`,
)

// ParseVersion parses s as a PEP 440 version.
func ParseVersion(s string) (Version, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return Version{}, fmt.Errorf("pypi: empty version string")
	}

	m := pep440Re.FindStringSubmatch(raw)
	if m == nil {
		return Version{}, fmt.Errorf("pypi: invalid PEP 440 version %q", s)
	}
	// subgroup indices:
	// 1: epoch, 2: release, 3: pre-label, 4: pre-num,
	// 5: post-label (post/rev/r), 6: post-num, 7: bare-post-num,
	// 9: dev keyword, 10: dev-num, 11: local
	// (group 8 is the inner capture inside the bare -N alternation)

	var v Version

	if m[1] != "" {
		e, err := strconv.Atoi(m[1])
		if err != nil {
			return Version{}, fmt.Errorf("pypi: bad epoch in %q", s)
		}
		v.Epoch = e
	}

	relParts := strings.Split(m[2], ".")
	v.Release = make([]int, len(relParts))
	for i, p := range relParts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return Version{}, fmt.Errorf("pypi: bad release segment %q in %q", p, s)
		}
		v.Release[i] = n
	}

	if m[3] != "" {
		label := normPreLabel(strings.ToLower(m[3]))
		n := 0
		if m[4] != "" {
			var err error
			n, err = strconv.Atoi(m[4])
			if err != nil {
				return Version{}, fmt.Errorf("pypi: bad pre-release number in %q", s)
			}
		}
		v.Pre = &preRelease{Label: label, N: n}
	}

	// post: either the named form (post/rev/r + optional N) or bare -N
	if m[5] != "" {
		n := 0
		if m[6] != "" {
			var err error
			n, err = strconv.Atoi(m[6])
			if err != nil {
				return Version{}, fmt.Errorf("pypi: bad post number in %q", s)
			}
		}
		v.Post = &n
	} else if m[7] != "" {
		n, err := strconv.Atoi(m[7])
		if err != nil {
			return Version{}, fmt.Errorf("pypi: bad bare post number in %q", s)
		}
		v.Post = &n
	}

	if m[9] != "" {
		n := 0
		if m[10] != "" {
			var err error
			n, err = strconv.Atoi(m[10])
			if err != nil {
				return Version{}, fmt.Errorf("pypi: bad dev number in %q", s)
			}
		}
		v.Dev = &n
	}

	if m[11] != "" {
		segs, err := parseLocal(m[11])
		if err != nil {
			return Version{}, fmt.Errorf("pypi: bad local version in %q: %w", s, err)
		}
		v.Local = segs
	}

	return v, nil
}

func normPreLabel(s string) string {
	switch s {
	case "alpha":
		return "a"
	case "beta":
		return "b"
	case "c", "pre", "preview":
		return "rc"
	default:
		return s
	}
}

func parseLocal(s string) ([]localSegment, error) {
	// separators in local are ., -, _
	parts := regexp.MustCompile(`[._-]`).Split(strings.ToLower(s), -1)
	segs := make([]localSegment, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("empty local segment")
		}
		if n, err := strconv.Atoi(p); err == nil {
			segs = append(segs, localSegment{IsNum: true, Num: n})
		} else {
			segs = append(segs, localSegment{IsNum: false, Str: p})
		}
	}
	return segs, nil
}

// String returns the normalized canonical PEP 440 form.
func (v Version) String() string {
	var b strings.Builder
	if v.Epoch != 0 {
		fmt.Fprintf(&b, "%d!", v.Epoch)
	}
	strs := make([]string, len(v.Release))
	for i, n := range v.Release {
		strs[i] = strconv.Itoa(n)
	}
	b.WriteString(strings.Join(strs, "."))
	if v.Pre != nil {
		fmt.Fprintf(&b, "%s%d", v.Pre.Label, v.Pre.N)
	}
	if v.Post != nil {
		fmt.Fprintf(&b, ".post%d", *v.Post)
	}
	if v.Dev != nil {
		fmt.Fprintf(&b, ".dev%d", *v.Dev)
	}
	if len(v.Local) > 0 {
		b.WriteByte('+')
		for i, seg := range v.Local {
			if i > 0 {
				b.WriteByte('.')
			}
			if seg.IsNum {
				b.WriteString(strconv.Itoa(seg.Num))
			} else {
				b.WriteString(seg.Str)
			}
		}
	}
	return b.String()
}

// IsPrerelease reports true when the version has a pre or dev component.
// Post-only versions are not pre-releases per PEP 440.
func (v Version) IsPrerelease() bool {
	return v.Pre != nil || v.Dev != nil
}

// Compare returns -1, 0, or +1. Panics if other is not a pypi.Version.
func (v Version) Compare(other ecosystem.Version) int {
	o, ok := other.(Version)
	if !ok {
		panic(fmt.Sprintf("pypi: cannot compare pypi.Version with %T", other))
	}
	return v.cmp(o)
}

// cmp implements the PEP 440 _cmpkey ordering.
func (v Version) cmp(o Version) int {
	if c := cmpInt(v.Epoch, o.Epoch); c != 0 {
		return c
	}
	if c := cmpRelease(v.Release, o.Release); c != 0 {
		return c
	}
	if c := cmpPreKey(v, o); c != 0 {
		return c
	}
	if c := cmpPostKey(v, o); c != 0 {
		return c
	}
	if c := cmpDevKey(v, o); c != 0 {
		return c
	}
	return cmpLocalKey(v.Local, o.Local)
}

// cmpRelease compares two release tuples, zero-padding the shorter one.
func cmpRelease(a, b []int) int {
	// strip trailing zeros for comparison
	a = stripTrailingZeros(a)
	b = stripTrailingZeros(b)
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if c := cmpInt(av, bv); c != 0 {
			return c
		}
	}
	return 0
}

func stripTrailingZeros(s []int) []int {
	end := len(s)
	for end > 0 && s[end-1] == 0 {
		end--
	}
	return s[:end]
}

// preLabel ordering: a=0, b=1, rc=2
func preLabelOrd(label string) int {
	switch label {
	case "a":
		return 0
	case "b":
		return 1
	default: // rc
		return 2
	}
}

// cmpPreKey encodes the packaging._cmpkey pre sentinel logic:
//   - dev-only (no pre, no post): NegativeInfinity
//   - has pre: (labelOrd, N)
//   - no pre (with or without post): Infinity
func cmpPreKey(v, o Version) int {
	vk := preKeyOf(v)
	ok := preKeyOf(o)
	return cmpTristate(vk, ok)
}

// preKeyOf returns -2 (neg-inf), -1 (has-pre sentinel), or +2 (inf).
// For has-pre we use a special sentinel and compare (labelOrd, N) separately.
// We use a three-level encoding via a struct approach with cmpTristate.
//
// Returns: kind -2=neginf, 0=has-pre, 2=inf; plus labelOrd and N for kind=0.
type preKey struct {
	kind     int // -2, 0, or 2
	labelOrd int
	n        int
}

func preKeyOf(v Version) preKey {
	if v.Pre != nil {
		return preKey{kind: 0, labelOrd: preLabelOrd(v.Pre.Label), n: v.Pre.N}
	}
	// dev-only with no pre and no post → negative infinity
	if v.Dev != nil && v.Post == nil {
		return preKey{kind: -2}
	}
	return preKey{kind: 2}
}

func cmpTristate(a, b preKey) int {
	if a.kind != b.kind {
		return cmpInt(a.kind, b.kind)
	}
	if a.kind != 0 {
		return 0 // both neginf or both inf
	}
	if c := cmpInt(a.labelOrd, b.labelOrd); c != 0 {
		return c
	}
	return cmpInt(a.n, b.n)
}

// cmpPostKey: no post → NegativeInfinity; post present → ("post", N).
func cmpPostKey(v, o Version) int {
	av := postKeyOf(v)
	bv := postKeyOf(o)
	if av[0] != bv[0] {
		return cmpInt(av[0], bv[0])
	}
	return cmpInt(av[1], bv[1])
}

func postKeyOf(v Version) [2]int {
	if v.Post == nil {
		return [2]int{-1, 0} // NegativeInfinity
	}
	return [2]int{0, *v.Post}
}

// cmpDevKey: no dev → Infinity; dev present → ("dev", N).
func cmpDevKey(v, o Version) int {
	av := devKeyOf(v)
	bv := devKeyOf(o)
	if av[0] != bv[0] {
		return cmpInt(av[0], bv[0])
	}
	return cmpInt(av[1], bv[1])
}

func devKeyOf(v Version) [2]int {
	if v.Dev == nil {
		return [2]int{1, 0} // Infinity
	}
	return [2]int{0, *v.Dev}
}

// cmpLocalKey: no local → NegativeInfinity; local present → segment-by-segment.
// Numeric segments > alpha segments; numeric by value; alpha lexicographic.
func cmpLocalKey(a, b []localSegment) int {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 {
		return -1
	}
	if len(b) == 0 {
		return 1
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if c := cmpLocalSeg(a[i], b[i]); c != 0 {
			return c
		}
	}
	return cmpInt(len(a), len(b))
}

func cmpLocalSeg(a, b localSegment) int {
	switch {
	case a.IsNum && b.IsNum:
		return cmpInt(a.Num, b.Num)
	case a.IsNum: // numeric > alpha
		return 1
	case b.IsNum:
		return -1
	default:
		return strings.Compare(a.Str, b.Str)
	}
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
