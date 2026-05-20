package pypi

import (
	"github.com/thgrace/semver-explode/pkg/ecosystem"
)

type Pypi struct {
	reg ecosystem.Registry
}

var _ ecosystem.Ecosystem = (*Pypi)(nil)

func New() *Pypi {
	return &Pypi{reg: NewRegistry()}
}

func (p *Pypi) Name() string { return "pypi" }

func (p *Pypi) ParseVersion(s string) (ecosystem.Version, error) {
	v, err := ParseVersion(s)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (p *Pypi) ParseRange(s string) (ecosystem.Range, error) {
	r, err := ParseRange(s)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (p *Pypi) Registry() ecosystem.Registry { return p.reg }

func init() {
	ecosystem.Register("pypi", func() ecosystem.Ecosystem { return New() })
}
