package ecosystem

// Op is the set of comparison operators shared across all ecosystem range
// languages. Each ecosystem may layer additional ops on top of these.
type Op int

const (
	OpEQ Op = iota
	OpNE
	OpLT
	OpLE
	OpGT
	OpGE
)

// MatchOrdered evaluates the six standard ordering ops using a sign value
// where sign == cmp(candidate, constraint). It returns false for any op
// not in the shared set — callers handle their extra ops before delegating.
func MatchOrdered(o Op, sign int) bool {
	switch o {
	case OpEQ:
		return sign == 0
	case OpNE:
		return sign != 0
	case OpLT:
		return sign < 0
	case OpLE:
		return sign <= 0
	case OpGT:
		return sign > 0
	case OpGE:
		return sign >= 0
	}
	return false
}
