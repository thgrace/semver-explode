package vers

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
)

// Sentinel errors returned by Parse and Bind.
var (
	ErrUnsupportedType = errors.New("vers: unsupported type")
	ErrTypeMismatch    = errors.New("vers: type does not match ecosystem")
	ErrNonCanonical    = errors.New("vers: constraints not in canonical order")
	ErrContradictory   = errors.New("vers: contradictory comparator sequence")
)

type Range struct {
	Type        string
	Constraints []Constraint
}

type Constraint struct {
	Op      Op
	Version string
}

type Op uint8

const (
	Eq Op = iota
	Ne
	Lt
	Le
	Gt
	Ge
	Star
)

var typeRe = regexp.MustCompile(`^[a-z][a-z0-9.-]*$`)

var typeAliases = map[string]string{
	"golang": "go",
	"gem":    "rubygems",
}

func Parse(s string) (Range, error) {
	if !strings.HasPrefix(s, "vers:") {
		return Range{}, fmt.Errorf("vers: missing \"vers:\" prefix")
	}
	s = strings.TrimPrefix(s, "vers:")

	slash := strings.IndexByte(s, '/')
	if slash < 0 {
		return Range{}, fmt.Errorf("vers: missing '/' between type and constraints")
	}
	typ := s[:slash]
	constraints := s[slash+1:]

	if typ == "" {
		return Range{}, fmt.Errorf("vers: empty type")
	}
	if constraints == "" {
		return Range{}, fmt.Errorf("vers: empty constraints")
	}

	if !typeRe.MatchString(typ) {
		return Range{}, fmt.Errorf("vers: invalid type %q", typ)
	}

	// Fix 3: reject any Unicode whitespace in constraints.
	for _, ch := range constraints {
		if unicode.IsSpace(ch) {
			return Range{}, fmt.Errorf("vers: whitespace in constraints")
		}
	}

	if strings.HasPrefix(constraints, "|") {
		return Range{}, fmt.Errorf("vers: leading '|' in constraints")
	}
	if strings.HasSuffix(constraints, "|") {
		return Range{}, fmt.Errorf("vers: trailing '|' in constraints")
	}
	if strings.Contains(constraints, "||") {
		return Range{}, fmt.Errorf("vers: double '|' in constraints")
	}

	parts := strings.Split(constraints, "|")
	parsed := make([]Constraint, 0, len(parts))

	for _, part := range parts {
		c, err := parseConstraint(part)
		if err != nil {
			return Range{}, err
		}
		parsed = append(parsed, c)
	}

	if len(parsed) > 1 {
		for _, c := range parsed {
			if c.Op == Star {
				return Range{}, fmt.Errorf("vers: '*' cannot be mixed with other constraints")
			}
		}
	}

	return Range{Type: typ, Constraints: parsed}, nil
}

func parseConstraint(s string) (Constraint, error) {
	if s == "*" {
		return Constraint{Op: Star, Version: ""}, nil
	}

	var op Op
	var rest string

	switch {
	case strings.HasPrefix(s, ">="):
		op = Ge
		rest = s[2:]
	case strings.HasPrefix(s, "<="):
		op = Le
		rest = s[2:]
	case strings.HasPrefix(s, "!="):
		op = Ne
		rest = s[2:]
	case strings.HasPrefix(s, ">"):
		op = Gt
		rest = s[1:]
	case strings.HasPrefix(s, "<"):
		op = Lt
		rest = s[1:]
	case strings.HasPrefix(s, "="):
		op = Eq
		rest = s[1:]
	default:
		op = Eq
		rest = s
	}

	if strings.ContainsAny(rest, "<>=!*|") {
		return Constraint{}, fmt.Errorf("vers: malformed version literal %q (contains raw reserved characters)", rest)
	}

	decoded, err := url.PathUnescape(rest)
	if err != nil {
		return Constraint{}, fmt.Errorf("vers: malformed percent-encoding in version %q: %w", rest, err)
	}
	if decoded == "" {
		return Constraint{}, fmt.Errorf("vers: empty version literal")
	}

	return Constraint{Op: op, Version: decoded}, nil
}

func opString(op Op) string {
	switch op {
	case Eq:
		return ""
	case Ne:
		return "!="
	case Lt:
		return "<"
	case Le:
		return "<="
	case Gt:
		return ">"
	case Ge:
		return ">="
	case Star:
		return "*"
	}
	return ""
}

func (r Range) String() string {
	parts := make([]string, 0, len(r.Constraints))
	for _, c := range r.Constraints {
		if c.Op == Star {
			parts = append(parts, "*")
		} else {
			parts = append(parts, opString(c.Op)+encodeVersionLiteral(c.Version))
		}
	}
	return "vers:" + r.Type + "/" + strings.Join(parts, "|")
}

type parsedItem struct {
	c   Constraint
	ver ecosystem.Version
}

func (r Range) Bind(eco ecosystem.Ecosystem) (ecosystem.Range, error) {
	resolved, err := resolveType(r.Type, eco)
	if err != nil {
		return nil, err
	}
	if resolved != eco.Name() {
		return nil, fmt.Errorf("vers: type %q does not match ecosystem %q: %w", r.Type, eco.Name(), ErrTypeMismatch)
	}

	if len(r.Constraints) == 0 {
		return &boundRange{eco: eco}, nil
	}
	if len(r.Constraints) == 1 && r.Constraints[0].Op == Star {
		return &boundRange{eco: eco, star: true}, nil
	}

	items := make([]parsedItem, len(r.Constraints))
	for i, c := range r.Constraints {
		v, err := eco.ParseVersion(c.Version)
		if err != nil {
			return nil, fmt.Errorf("vers: parse version %q: %w", c.Version, err)
		}
		items[i] = parsedItem{c: c, ver: v}
	}

	sorted := make([]parsedItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		cmp := sorted[i].ver.Compare(sorted[j].ver)
		if cmp != 0 {
			return cmp < 0
		}
		return sorted[i].c.Op < sorted[j].c.Op
	})

	for i := 1; i < len(sorted); i++ {
		if sorted[i-1].ver.Compare(sorted[i].ver) == 0 {
			return nil, fmt.Errorf("vers: duplicate version %q: %w", sorted[i].c.Version, ErrContradictory)
		}
	}

	for i, item := range items {
		if item.ver.Compare(sorted[i].ver) != 0 || item.c.Op != sorted[i].c.Op {
			return nil, fmt.Errorf("vers: constraints are not in canonical order: %w", ErrNonCanonical)
		}
	}

	if err := validateComparators(sorted); err != nil {
		return nil, err
	}

	sortedConstraints := make([]Constraint, len(sorted))
	sortedVersions := make([]ecosystem.Version, len(sorted))
	for i, item := range sorted {
		sortedConstraints[i] = item.c
		sortedVersions[i] = item.ver
	}

	return &boundRange{
		eco:         eco,
		constraints: sortedConstraints,
		versions:    sortedVersions,
	}, nil
}

// resolveType returns the canonical ecosystem name for typ, or an error if the
// type is an alias whose target is not registered in the global ecosystem registry.
// Fix 6: alias target registration check — only applies when the alias target
// does not equal eco.Name() (i.e. we can't rely on eco itself as proof of registration).
func resolveType(typ string, eco ecosystem.Ecosystem) (string, error) {
	if typ == eco.Name() {
		return eco.Name(), nil
	}
	if alias, ok := typeAliases[typ]; ok {
		if alias == eco.Name() {
			// The ecosystem we are binding against IS the alias target — it's
			// registered by definition.
			return eco.Name(), nil
		}
		// Fix 6: alias target differs from eco; verify it is registered.
		if _, registered := ecosystem.Lookup(alias); !registered {
			return "", fmt.Errorf("vers: unsupported type %q (alias target %q not registered): %w", typ, alias, ErrUnsupportedType)
		}
	}
	return typ, nil
}

func validateComparators(items []parsedItem) error {
	for i := 0; i+1 < len(items); i++ {
		if !validAdjacent(items[i].c.Op, items[i+1].c.Op) {
			return fmt.Errorf("vers: contradictory comparator sequence: %w", ErrContradictory)
		}
	}

	withoutNe := make([]Op, 0, len(items))
	for _, item := range items {
		if item.c.Op != Ne {
			withoutNe = append(withoutNe, item.c.Op)
		}
	}

	for i, op := range withoutNe {
		if op != Eq {
			continue
		}
		if i > 0 {
			prev := withoutNe[i-1]
			if prev == Gt || prev == Ge {
				return fmt.Errorf("vers: contradictory comparator sequence: %w", ErrContradictory)
			}
		}
		if i+1 == len(withoutNe) {
			continue
		}
		next := withoutNe[i+1]
		if next != Eq && next != Gt && next != Ge {
			return fmt.Errorf("vers: contradictory comparator sequence: %w", ErrContradictory)
		}
	}

	withoutEqNe := make([]Op, 0, len(withoutNe))
	for _, op := range withoutNe {
		if op != Eq {
			withoutEqNe = append(withoutEqNe, op)
		}
	}

	for i, op := range withoutEqNe {
		if i+1 == len(withoutEqNe) {
			continue
		}
		next := withoutEqNe[i+1]
		switch op {
		case Lt, Le:
			if next != Gt && next != Ge {
				return fmt.Errorf("vers: contradictory comparator sequence: %w", ErrContradictory)
			}
		case Gt, Ge:
			if next != Lt && next != Le {
				return fmt.Errorf("vers: contradictory comparator sequence: %w", ErrContradictory)
			}
		}
	}
	return nil
}

func validAdjacent(current, next Op) bool {
	switch current {
	case Ne:
		return true
	case Eq, Lt, Le:
		return next == Eq || next == Ne || next == Gt || next == Ge
	case Gt, Ge:
		return next == Ne || next == Lt || next == Le
	default:
		return false
	}
}

func encodeVersionLiteral(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if strings.ContainsAny(string(c), "%<>=!*|") || c <= ' ' || c >= 0x7f {
			fmt.Fprintf(&b, "%%%02X", c)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
