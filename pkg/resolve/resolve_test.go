package resolve

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/thgrace/semver-explode/pkg/ecosystem"
	"github.com/thgrace/semver-explode/pkg/npm"
)

// fakeRegistry is an in-memory ecosystem.Registry for tests.
type fakeRegistry struct {
	versions []ecosystem.Version
	err      error
}

func (r *fakeRegistry) ListVersions(_ context.Context, _ string) ([]ecosystem.Version, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.versions, nil
}

// fakeEco wraps an ecosystem but overrides the registry.
type fakeEco struct {
	ecosystem.Ecosystem
	reg ecosystem.Registry
}

func (f *fakeEco) Registry() ecosystem.Registry { return f.reg }

func npmVersions(t *testing.T, strs ...string) []ecosystem.Version {
	t.Helper()
	vs := make([]ecosystem.Version, len(strs))
	for i, s := range strs {
		v, err := npm.ParseVersion(s)
		if err != nil {
			t.Fatalf("npm.ParseVersion(%q): %v", s, err)
		}
		vs[i] = v
	}
	return vs
}

func TestResolveExact(t *testing.T) {
	ctx := context.Background()
	base := npm.New()

	t.Run("match_exact", func(t *testing.T) {
		reg := &fakeRegistry{versions: npmVersions(t, "1.0.0", "1.2.3", "2.0.0")}
		eco := &fakeEco{Ecosystem: base, reg: reg}

		got, err := ResolveExact(ctx, eco, "lodash", "1.2.3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.String() != "1.2.3" {
			t.Errorf("got %q, want %q", got.String(), "1.2.3")
		}
	})

	// npm normalizes "1.0" → "1.0.0"; the registry entry "1.0.0" must match.
	t.Run("normalized_equality", func(t *testing.T) {
		reg := &fakeRegistry{versions: npmVersions(t, "1.0.0", "2.0.0")}
		eco := &fakeEco{Ecosystem: base, reg: reg}

		// npm ParseVersion("1.0") succeeds and compares equal to "1.0.0".
		got, err := ResolveExact(ctx, eco, "lodash", "1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.String() != "1.0.0" {
			t.Errorf("got %q, want %q", got.String(), "1.0.0")
		}
	})

	t.Run("not_found", func(t *testing.T) {
		reg := &fakeRegistry{versions: npmVersions(t, "1.0.0", "2.0.0")}
		eco := &fakeEco{Ecosystem: base, reg: reg}

		_, err := ResolveExact(ctx, eco, "lodash", "3.0.0")
		if !errors.Is(err, ErrVersionNotFound) {
			t.Errorf("got %v, want ErrVersionNotFound", err)
		}
	})

	t.Run("registry_error", func(t *testing.T) {
		regErr := fmt.Errorf("registry unavailable")
		reg := &fakeRegistry{err: regErr}
		eco := &fakeEco{Ecosystem: base, reg: reg}

		_, err := ResolveExact(ctx, eco, "lodash", "1.0.0")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, regErr) {
			t.Errorf("got %v, want error wrapping %v", err, regErr)
		}
	})

	t.Run("parse_error", func(t *testing.T) {
		reg := &fakeRegistry{versions: npmVersions(t, "1.0.0")}
		eco := &fakeEco{Ecosystem: base, reg: reg}

		_, err := ResolveExact(ctx, eco, "lodash", "not-a-semver-version!!!")
		if err == nil {
			t.Fatal("expected parse error, got nil")
		}
		if errors.Is(err, ErrVersionNotFound) {
			t.Errorf("got ErrVersionNotFound, want a parse error")
		}
	})
}
