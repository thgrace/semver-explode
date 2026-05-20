package depsdev

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/thgrace/semver-explode/internal/httpx"
)

const DefaultBaseURL = "https://api.deps.dev"

// TODO(phase1): wire go-vcr here so registry tests can replay recorded
// fixtures from testdata/fixtures/<ecosystem>/<package>.yaml.

type Client struct {
	BaseURL string
	HTTP    *httpx.Client
}

func New() *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTP:    httpx.New(0),
	}
}

type VersionKey struct {
	System  string `json:"system"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type VersionInfo struct {
	VersionKey  VersionKey `json:"versionKey"`
	PublishedAt time.Time  `json:"publishedAt"`
	IsDefault   bool       `json:"isDefault"`
}

type Package struct {
	PackageKey struct {
		System string `json:"system"`
		Name   string `json:"name"`
	} `json:"packageKey"`
	Versions []VersionInfo `json:"versions"`
}

func (c *Client) GetPackage(ctx context.Context, system, name string) (*Package, error) {
	endpoint := fmt.Sprintf("%s/v3/systems/%s/packages/%s",
		c.BaseURL, url.PathEscape(system), url.PathEscape(name))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deps.dev: %s %s -> %s", system, name, resp.Status)
	}

	var pkg Package
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		return nil, fmt.Errorf("decode deps.dev response: %w", err)
	}
	return &pkg, nil
}
