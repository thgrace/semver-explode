package resolve

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
)

var ErrVersionNotFound = errors.New("version not found")

type SortOrder int

const (
	Ascending SortOrder = iota
	Descending
)

type config struct {
	includePrereleases bool
	sort               SortOrder
}

type Option func(*config)

func IncludePrereleases(v bool) Option {
	return func(c *config) { c.includePrereleases = v }
}

func Sort(order SortOrder) Option {
	return func(c *config) { c.sort = order }
}

func Resolve(ctx context.Context, eco ecosystem.Ecosystem, pkg, rangeStr string, opts ...Option) ([]ecosystem.Version, error) {
	cfg := config{sort: Ascending}
	for _, opt := range opts {
		opt(&cfg)
	}

	r, err := eco.ParseRange(rangeStr)
	if err != nil {
		return nil, err
	}

	all, err := eco.Registry().ListVersions(ctx, pkg)
	if err != nil {
		return nil, err
	}

	out := make([]ecosystem.Version, 0, len(all))
	for _, v := range all {
		if !cfg.includePrereleases && v.IsPrerelease() {
			continue
		}
		if r.Contains(v) {
			out = append(out, v)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		cmp := out[i].Compare(out[j])
		if cfg.sort == Descending {
			return cmp > 0
		}
		return cmp < 0
	})

	return out, nil
}

func ResolveExact(ctx context.Context, eco ecosystem.Ecosystem, pkg, versionStr string) (ecosystem.Version, error) {
	parsed, err := eco.ParseVersion(versionStr)
	if err != nil {
		return nil, fmt.Errorf("parse version %q: %w", versionStr, err)
	}

	all, err := eco.Registry().ListVersions(ctx, pkg)
	if err != nil {
		return nil, fmt.Errorf("list versions for %q: %w", pkg, err)
	}

	for _, v := range all {
		if v.Compare(parsed) == 0 {
			return v, nil
		}
	}
	return nil, ErrVersionNotFound
}
