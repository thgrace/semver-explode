package ecosystem

import "fmt"

// AssertType is a helper for per-ecosystem Compare methods that must
// panic when handed a Version of the wrong concrete type. It returns
// the asserted value, panicking with a clear ecosystem-prefixed message
// if other is not of type T.
//
// Per the conformance contract, each ecosystem's Version.Compare panics
// when given a Version from a different ecosystem — the interface is
// intentionally per-ecosystem, not cross-comparable.
func AssertType[T any](system string, other any) T {
	v, ok := other.(T)
	if !ok {
		panic(fmt.Sprintf("%s: cannot compare with %T", system, other))
	}
	return v
}
