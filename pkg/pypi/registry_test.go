package pypi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thgrace/semver-explode/internal/depsdev"
)

func TestRegistryListVersions(t *testing.T) {
	payload := depsdev.Package{
		Versions: []depsdev.VersionInfo{
			{VersionKey: depsdev.VersionKey{Version: "2.0"}},
			{VersionKey: depsdev.VersionKey{Version: "1.0"}},
			{VersionKey: depsdev.VersionKey{Version: "2.0a1"}},
			{VersionKey: depsdev.VersionKey{Version: "2.0.post1"}},
			{VersionKey: depsdev.VersionKey{Version: "not-a-version!!!"}},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := depsdev.New()
	c.BaseURL = srv.URL

	reg := NewRegistryWithClient(c)
	versions, err := reg.ListVersions(context.Background(), "test")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}

	// 4 parseable versions; the invalid one is skipped
	if len(versions) != 4 {
		t.Fatalf("want 4 versions, got %d", len(versions))
	}

	// Versions should be sorted ascending: 1.0 < 2.0a1 < 2.0 < 2.0.post1
	want := []string{"1.0", "2.0a1", "2.0", "2.0.post1"}
	for i, v := range versions {
		if v.String() != want[i] {
			t.Errorf("versions[%d]: want %q, got %q", i, want[i], v.String())
		}
	}
}
