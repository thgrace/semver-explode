package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
	_ "github.com/thgrace/semver-explode/pkg/ecosystem/all"
	"github.com/thgrace/semver-explode/pkg/purl"
	"github.com/thgrace/semver-explode/pkg/resolve"
	"github.com/thgrace/semver-explode/pkg/vers"
)

// Version is set at build time via -ldflags "-X main.Version=..."
var Version = "dev"

func usageString() string {
	return fmt.Sprintf(`usage:
  semver-explode <ecosystem> <package> <range>
  semver-explode <purl> <range>
  semver-explode <purl>              (purl must contain @version)

range may be a native ecosystem expression or a vers: range, e.g.:
  vers:npm/>=4.17.0|<5.0.0

ecosystems: %s

examples:
  semver-explode npm lodash '^4.17.0'
  semver-explode pypi requests '>=2.30,<3'
  semver-explode 'pkg:npm/lodash' '^4.17.0'
  semver-explode 'pkg:npm/lodash@4.17.21'
  semver-explode 'pkg:pypi/Django@4.2'
  semver-explode 'pkg:npm/lodash' 'vers:npm/>=4.17.0|<5.0.0'
`, strings.Join(ecosystem.Names(), ", "))
}

type request struct {
	ecoName    string
	pkgName    string
	rangeExpr  string
	pinVersion string
	mode       requestMode
}

type requestMode int

const (
	requestModeRange requestMode = iota
	requestModePin
)

func parseRequest(args []string) (request, error) {
	if len(args) == 0 {
		return request{}, fmt.Errorf("expected arguments, got 0")
	}

	first := args[0]

	if strings.HasPrefix(first, "pkg:") {
		p, err := purl.Parse(first)
		if err != nil {
			return request{}, err
		}
		ecoName := p.Ecosystem()
		if ecoName == "" {
			return request{}, fmt.Errorf("unsupported purl type %q", p.Type)
		}
		pkgName, err := p.PackageName()
		if err != nil {
			return request{}, err
		}
		switch len(args) {
		case 1:
			if p.Version == "" {
				return request{}, fmt.Errorf("purl has no @version; provide a range argument or pin a version with @version")
			}
			return request{ecoName: ecoName, pkgName: pkgName, pinVersion: p.Version, mode: requestModePin}, nil
		case 2:
			rangeExpr := args[1]
			if p.Version != "" {
				return request{}, fmt.Errorf("version pin and range both given")
			}
			return request{ecoName: ecoName, pkgName: pkgName, rangeExpr: rangeExpr, mode: requestModeRange}, nil
		default:
			return request{}, fmt.Errorf("too many arguments")
		}
	}

	if strings.HasPrefix(first, "vers:") {
		return request{}, fmt.Errorf("vers: range syntax is not a package selector")
	}

	if len(args) == 3 {
		return request{ecoName: args[0], pkgName: args[1], rangeExpr: args[2], mode: requestModeRange}, nil
	}

	return request{}, fmt.Errorf("expected 3 arguments, got %d", len(args))
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "semver-explode:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	return runWithDeps(args, os.Stdout, os.Stderr, ecosystem.Lookup)
}

func runWithDeps(args []string, stdout, stderr io.Writer, lookup func(string) (ecosystem.Ecosystem, bool)) error {
	if len(args) == 1 && (args[0] == "-v" || args[0] == "--version") {
		fmt.Fprintln(stdout, Version)
		return nil
	}

	req, err := parseRequest(args)
	if err != nil {
		fmt.Fprint(stderr, usageString())
		return err
	}

	eco, ok := lookup(req.ecoName)
	if !ok {
		return fmt.Errorf("unknown ecosystem %q (supported: %s)", req.ecoName, strings.Join(ecosystem.Names(), ", "))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	switch req.mode {
	case requestModeRange:
		var rng ecosystem.Range
		if strings.HasPrefix(req.rangeExpr, "vers:") {
			vr, err := vers.Parse(req.rangeExpr)
			if err != nil {
				return fmt.Errorf("parse vers range: %w", err)
			}
			rng, err = vr.Bind(eco)
			if err != nil {
				return fmt.Errorf("bind vers range: %w", err)
			}
		} else {
			var err error
			rng, err = eco.ParseRange(req.rangeExpr)
			if err != nil {
				return fmt.Errorf("parse range: %w", err)
			}
		}
		versions, err := eco.Registry().ListVersions(ctx, req.pkgName)
		if err != nil {
			return err
		}
		for _, v := range versions {
			if rng.Contains(v) {
				fmt.Fprintln(stdout, v.String())
			}
		}
	case requestModePin:
		v, err := resolve.ResolveExact(ctx, eco, req.pkgName, req.pinVersion)
		if err != nil {
			if errors.Is(err, resolve.ErrVersionNotFound) {
				return fmt.Errorf("version %q not found for package %q", req.pinVersion, req.pkgName)
			}
			return err
		}
		fmt.Fprintln(stdout, v.String())
	default:
		return fmt.Errorf("internal error: unknown request mode %d", req.mode)
	}

	return nil
}
