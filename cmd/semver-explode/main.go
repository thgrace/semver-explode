package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
	"github.com/thgrace/semver-explode/pkg/npm"
	"github.com/thgrace/semver-explode/pkg/pypi"
)

const usage = `usage: semver-explode <ecosystem> <package> <range>

ecosystems: npm, pypi

example:
  semver-explode npm lodash '^4.17.0'
  semver-explode pypi requests '>=2.30,<3'
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "semver-explode:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 3 {
		fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("expected 3 arguments, got %d", len(args))
	}
	ecoName, pkgName, rangeExpr := args[0], args[1], args[2]

	eco, err := lookupEcosystem(ecoName)
	if err != nil {
		return err
	}

	rng, err := eco.ParseRange(rangeExpr)
	if err != nil {
		return fmt.Errorf("parse range: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	versions, err := eco.Registry().ListVersions(ctx, pkgName)
	if err != nil {
		return err
	}

	for _, v := range versions {
		if rng.Contains(v) {
			fmt.Println(v.String())
		}
	}
	return nil
}

func lookupEcosystem(name string) (ecosystem.Ecosystem, error) {
	switch name {
	case "npm":
		return npm.New(), nil
	case "pypi":
		return pypi.New(), nil
	default:
		return nil, fmt.Errorf("unknown ecosystem %q (supported: npm, pypi)", name)
	}
}
