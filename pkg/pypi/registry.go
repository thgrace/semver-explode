package pypi

import (
	"github.com/thgrace/semver-explode/internal/depsdev"
	"github.com/thgrace/semver-explode/pkg/ecosystem"
)

type Registry = ecosystem.DepsDevRegistry[Version]

func NewRegistry() *Registry {
	return ecosystem.NewDepsDevRegistry("pypi", ParseVersion)
}

func NewRegistryWithClient(c *depsdev.Client) *Registry {
	return ecosystem.NewDepsDevRegistryWithClient("pypi", ParseVersion, c)
}
