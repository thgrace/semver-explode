// Package ecosystem defines the interfaces every per-ecosystem package under
// pkg/<eco> must satisfy, along with the conformance contract below.
//
// # Ecosystem conformance contract
//
// Each pkg/<eco> package must export the following symbols:
//
//	// New returns an Ecosystem backed by the default deps.dev registry.
//	func New() *<Eco>
//
//	// Name returns the lowercase deps.dev system name (e.g. "npm", "pypi").
//	func (e *<Eco>) Name() string
//
//	// ParseVersion parses a version string and returns the package-local
//	// Version struct (not the ecosystem.Version interface) so callers in
//	// the same package can use concrete-type methods.
//	func ParseVersion(s string) (Version, error)
//
//	// ParseRange parses a version-range expression and returns the
//	// package-local Range struct for the same reason as ParseVersion.
//	func ParseRange(s string) (Range, error)
//
//	// Version and Range must satisfy ecosystem.Version and ecosystem.Range.
//	type Version struct{ ... }
//	type Range   struct{ ... }
//
//	// NewRegistry returns a deps.dev-backed registry using default settings.
//	func NewRegistry() *Registry
//
//	// NewRegistryWithClient accepts a custom *depsdev.Client, used in tests.
//	func NewRegistryWithClient(c *depsdev.Client) *Registry
//
// The registry must be backed by deps.dev; that is the shared resolution
// backend for all ecosystems in this module.
package ecosystem

import (
	"context"
	"errors"
)

type Ecosystem interface {
	Name() string
	ParseVersion(s string) (Version, error)
	ParseRange(s string) (Range, error)
	Registry() Registry
}

type Version interface {
	String() string
	Compare(other Version) int
	IsPrerelease() bool
}

type Range interface {
	Contains(v Version) bool
	String() string
}

type Registry interface {
	ListVersions(ctx context.Context, pkg string) ([]Version, error)
}

var ErrNotImplemented = errors.New("not implemented")
