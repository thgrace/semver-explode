package npm

import (
	"github.com/thgrace/semver-explode/pkg/ecosystem"
)

type Npm struct {
	reg ecosystem.Registry
}

var _ ecosystem.Ecosystem = (*Npm)(nil)

func New() *Npm {
	return &Npm{reg: NewRegistry()}
}

func (n *Npm) Name() string { return "npm" }

func (n *Npm) ParseVersion(s string) (ecosystem.Version, error) {
	v, err := ParseVersion(s)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (n *Npm) ParseRange(s string) (ecosystem.Range, error) {
	r, err := ParseRange(s)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (n *Npm) Registry() ecosystem.Registry { return n.reg }
