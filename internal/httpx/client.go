package httpx

import (
	"net/http"
	"time"
)

const (
	DefaultUserAgent = "semver-explode/0.1.0"
	defaultTimeout   = 30 * time.Second
	maxAttempts      = 3
	backoffBase      = 200 * time.Millisecond
)

type Client struct {
	HTTP      *http.Client
	UserAgent string
}

func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{
		HTTP:      &http.Client{Timeout: timeout},
		UserAgent: DefaultUserAgent,
	}
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	var (
		resp *http.Response
		err  error
	)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(backoffBase * (1 << (attempt - 1))):
			}
		}
		resp, err = c.HTTP.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}
		resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	return resp, nil
}
