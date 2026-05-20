package vers

import (
	"strings"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
)

type boundRange struct {
	eco         ecosystem.Ecosystem
	constraints []Constraint        // sorted by version (from Bind)
	versions    []ecosystem.Version // parallel to constraints; nil when star==true
	star        bool
}

func (br *boundRange) String() string {
	if br.star {
		return "vers:" + br.eco.Name() + "/*"
	}
	parts := make([]string, len(br.constraints))
	for i, c := range br.constraints {
		parts[i] = opString(c.Op) + c.Version
	}
	return "vers:" + br.eco.Name() + "/" + strings.Join(parts, "|")
}

// Contains implements the pairwise-walk algorithm from the vers spec.
//
// The constraints are already sorted by version. We walk them in order,
// tracking whether v falls inside an open interval. Comparators Ge/Gt open
// an interval; Le/Lt close one. Eq/Ne are point-in / point-out tests that
// short-circuit.
func (br *boundRange) Contains(v ecosystem.Version) bool {
	if br.star {
		return true
	}
	if len(br.constraints) == 0 {
		return false
	}

	inRange := false
	for i, c := range br.constraints {
		pv := br.versions[i]
		cmp := v.Compare(pv)

		switch c.Op {
		case Eq:
			if cmp == 0 {
				return true
			}
		case Ne:
			if cmp == 0 {
				return false
			}
		case Ge:
			if cmp >= 0 {
				inRange = true
			}
		case Gt:
			if cmp > 0 {
				inRange = true
			}
		case Le:
			if inRange && cmp <= 0 {
				return true
			}
			inRange = false
		case Lt:
			if inRange && cmp < 0 {
				return true
			}
			inRange = false
		}
	}

	return inRange
}
