// Package all is a convenience import that registers every implemented
// ecosystem with pkg/ecosystem. Import it for its side effects when you
// want all ecosystems available to ecosystem.Lookup:
//
//	import _ "github.com/thgrace/semver-explode/pkg/ecosystem/all"
//
// If you only need a specific ecosystem, blank-import that package
// directly instead — this package pulls in every implemented ecosystem
// and its transitive deps (currently the deps.dev HTTP client).
package all

import (
	_ "github.com/thgrace/semver-explode/pkg/npm"
	_ "github.com/thgrace/semver-explode/pkg/pypi"
)

// When adding a new ecosystem, blank-import it above.
