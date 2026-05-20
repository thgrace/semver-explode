package ecosystem

import (
	"context"
	"fmt"
	"sort"

	"github.com/thgrace/semver-explode/internal/depsdev"
)

// DepsDevRegistry is a generic deps.dev-backed registry that can be shared
// across per-ecosystem packages. V must satisfy ecosystem.Version.
type DepsDevRegistry[V Version] struct {
	Client *depsdev.Client
	System string // e.g. "npm", "pypi"
	Parse  func(string) (V, error)
}

// ListVersions fetches all versions of pkg from deps.dev, skips unparseable
// entries, sorts the result ascending, and returns it as []ecosystem.Version.
func (r *DepsDevRegistry[V]) ListVersions(ctx context.Context, pkg string) ([]Version, error) {
	p, err := r.Client.GetPackage(ctx, r.System, pkg)
	if err != nil {
		return nil, fmt.Errorf("%s: list versions for %q: %w", r.System, pkg, err)
	}

	out := make([]V, 0, len(p.Versions))
	skipped := 0
	for _, vi := range p.Versions {
		v, err := r.Parse(vi.VersionKey.Version)
		if err != nil {
			skipped++
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 && skipped > 0 {
		return nil, fmt.Errorf("%s: no parseable versions for %q (%d skipped)", r.System, pkg, skipped)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Compare(out[j]) < 0 })

	res := make([]Version, len(out))
	for i, v := range out {
		res[i] = v
	}
	return res, nil
}

// NewDepsDevRegistry returns a DepsDevRegistry backed by the default
// deps.dev client. Prefer this over constructing DepsDevRegistry directly.
func NewDepsDevRegistry[V Version](system string, parse func(string) (V, error)) *DepsDevRegistry[V] {
	return NewDepsDevRegistryWithClient(system, parse, depsdev.New())
}

// NewDepsDevRegistryWithClient returns a DepsDevRegistry using the provided
// client. Useful in tests where a custom or mock client is needed.
func NewDepsDevRegistryWithClient[V Version](system string, parse func(string) (V, error), c *depsdev.Client) *DepsDevRegistry[V] {
	return &DepsDevRegistry[V]{Client: c, System: system, Parse: parse}
}
