// Package gdxapi is a client for gdx's debug HTTP surface (see gdx.NewHTTPFn).
package gdxapi

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// NewUnixClient returns an http.Client that dials the gdx debug socket at path.
func NewUnixClient(path string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", path)
			},
		},
	}
}

// Fetch issues a GET against urlPath?duration=<seconds> (converting duration
// to whole seconds, the only unit gdx/server.go's parseDuration understands)
// and streams a successful response to out.
func Fetch(ctx context.Context, c *http.Client, urlPath string, duration time.Duration, out io.Writer) error {
	seconds := int(duration.Seconds())

	ctx, cancel := context.WithTimeout(ctx, duration+10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://unix%s?duration=%d", urlPath, seconds), nil)
	if err != nil {
		return fmt.Errorf("unable to build request: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("unable to reach gdx debug socket: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gdx debug socket returned %d: %s", resp.StatusCode, string(body))
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("unable to write response: %w", err)
	}

	return nil
}
