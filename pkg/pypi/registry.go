package pypi

import (
	"context"
	"fmt"
	"sort"

	"github.com/thgrace/semver-explode/internal/depsdev"
	"github.com/thgrace/semver-explode/pkg/ecosystem"
)

type Registry struct {
	client *depsdev.Client
}

var _ ecosystem.Registry = (*Registry)(nil)

func NewRegistry() *Registry {
	return &Registry{client: depsdev.New()}
}

func NewRegistryWithClient(c *depsdev.Client) *Registry {
	return &Registry{client: c}
}

func (r *Registry) ListVersions(ctx context.Context, pkg string) ([]ecosystem.Version, error) {
	p, err := r.client.GetPackage(ctx, "pypi", pkg)
	if err != nil {
		return nil, fmt.Errorf("pypi: list versions for %q: %w", pkg, err)
	}

	out := make([]Version, 0, len(p.Versions))
	skipped := 0
	for _, vi := range p.Versions {
		v, err := ParseVersion(vi.VersionKey.Version)
		if err != nil {
			skipped++
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 && skipped > 0 {
		return nil, fmt.Errorf("pypi: no parseable versions for %q (%d skipped)", pkg, skipped)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].cmp(out[j]) < 0 })

	res := make([]ecosystem.Version, len(out))
	for i, v := range out {
		res[i] = v
	}
	return res, nil
}
