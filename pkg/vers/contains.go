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
// Steps:
//  1. Star → true.
//  2. Empty → false.
//  3. Walk all constraints: handle exact-version matches for each operator.
//     Eq exact → true. Ne exact → false.
//     Le/Ge exact → true (inclusive boundary). Lt/Gt exact → false (exclusive boundary).
//  4. Find prevIdx (largest i with Compare(versions[i], v) < 0, skipping Ne) and
//     nextIdx (smallest i with Compare(versions[i], v) > 0, skipping Ne).
//  5. If no non-Ne bounding constraints exist → true (unbounded, no exclusion hit in step 3).
//  6. If prevIdx == -1: v is below all bounding constraints; return next == Lt || next == Le.
//  7. If nextIdx == len: v is above all bounding constraints; return prev == Gt || prev == Ge.
//  8. Otherwise: return (prev == Gt || prev == Ge) && (next == Lt || next == Le).
func (br *boundRange) Contains(v ecosystem.Version) bool {
	if br.star {
		return true
	}
	if len(br.constraints) == 0 {
		return false
	}

	// Step 3: exact-version short-circuits for all operators.
	hasBoundingConstraint := false
	for i, c := range br.constraints {
		cmp := v.Compare(br.versions[i])
		if cmp != 0 {
			if c.Op != Ne {
				hasBoundingConstraint = true
			}
			continue
		}
		// cmp == 0
		switch c.Op {
		case Eq, Le, Ge:
			return true
		case Ne:
			return false
		case Lt, Gt:
			return false
		}
	}

	// If all constraints were Ne and none matched, v is in the open set.
	if !hasBoundingConstraint {
		return true
	}

	// Steps 4–8: neighbor search (skip Ne constraints; cmp==0 already handled above).
	prevIdx := -1
	nextIdx := len(br.constraints)

	for i, c := range br.constraints {
		if c.Op == Ne {
			continue
		}
		cmp := v.Compare(br.versions[i])
		if cmp > 0 {
			prevIdx = i
		} else if cmp < 0 && i < nextIdx {
			nextIdx = i
		}
	}

	switch {
	case prevIdx == -1 && nextIdx == len(br.constraints):
		// Only Ne constraints (already handled) or truly empty bounding set.
		return false
	case prevIdx == -1:
		// v is below all bounding constraints.
		next := br.constraints[nextIdx].Op
		return next == Lt || next == Le
	case nextIdx == len(br.constraints):
		// v is above all bounding constraints.
		prev := br.constraints[prevIdx].Op
		return prev == Gt || prev == Ge
	default:
		// v is strictly between two bounding constraints.
		prev := br.constraints[prevIdx].Op
		next := br.constraints[nextIdx].Op
		return (prev == Gt || prev == Ge) && (next == Lt || next == Le)
	}
}
