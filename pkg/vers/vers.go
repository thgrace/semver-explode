package vers

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
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

var typeRe = regexp.MustCompile(`^[a-z][a-z0-9.\-]*$`)

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

	typ = strings.ToLower(typ)
	if !typeRe.MatchString(typ) {
		return Range{}, fmt.Errorf("vers: invalid type %q", typ)
	}

	for _, ch := range constraints {
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
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

	seen := map[string]bool{}
	for _, c := range parsed {
		key := fmt.Sprintf("%d:%s", c.Op, c.Version)
		if seen[key] {
			return Range{}, fmt.Errorf("vers: duplicate constraint %q", key)
		}
		seen[key] = true
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
			parts = append(parts, opString(c.Op)+c.Version)
		}
	}
	return "vers:" + r.Type + "/" + strings.Join(parts, "|")
}

type parsedItem struct {
	c   Constraint
	ver ecosystem.Version
}

func (r Range) Bind(eco ecosystem.Ecosystem) (ecosystem.Range, error) {
	resolved := resolveType(r.Type, eco)
	if resolved != eco.Name() {
		return nil, fmt.Errorf("vers: type %q does not match ecosystem %q", r.Type, eco.Name())
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

	for i, item := range items {
		if item.ver.Compare(sorted[i].ver) != 0 || item.c.Op != sorted[i].c.Op {
			return nil, fmt.Errorf("vers: constraints are not in canonical order")
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

func resolveType(typ string, eco ecosystem.Ecosystem) string {
	if typ == eco.Name() {
		return eco.Name()
	}
	if alias, ok := typeAliases[typ]; ok && alias == eco.Name() {
		return eco.Name()
	}
	return typ
}

func validateComparators(items []parsedItem) error {
	inRange := false
	for _, item := range items {
		switch item.c.Op {
		case Ge, Gt:
			if inRange {
				return fmt.Errorf("vers: contradictory comparator sequence")
			}
			inRange = true
		case Le, Lt:
			inRange = false
		}
	}
	return nil
}
