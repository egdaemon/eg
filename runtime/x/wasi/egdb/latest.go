package egdb

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/jsonx"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
)

// envAccessToken is the access token the runner provides to the workload for the control plane.
const envAccessToken = "EG_GIT_AUTH_ACCESS_TOKEN"

type latest struct {
	workload string
	endpoint string
	token    string
	c        *http.Client
}

// LatestOption configures [Latest].
type LatestOption func(*latest)

// WithWorkload selects the workload (module name, see eg.DefaultWorkload) to find the last successful run of.
// defaults to the workload currently running.
func WithWorkload(entry string) LatestOption {
	return func(l *latest) {
		l.workload = entry
	}
}

// WithEndpoint overrides the base url of the control plane api.
func WithEndpoint(endpoint string) LatestOption {
	return func(l *latest) {
		l.endpoint = endpoint
	}
}

// WithClient overrides the http client used to talk to the control plane.
func WithClient(c *http.Client) LatestOption {
	return func(l *latest) {
		l.c = c
	}
}

// Latest provides the path of a local copy of the analytics database of the last successful run of the workload.
// the copy is stored in the cache directory, when it already exists it is returned as is, otherwise it is downloaded
// from the control plane. see [Cache] for keeping the copy current.
//
// returns an error matching os.ErrNotExist when the workload has no successful runs.
func Latest(ctx context.Context, options ...LatestOption) (_ string, err error) {
	l := langx.Clone(latest{
		workload: egenv.WorkloadName(),
		endpoint: eg.EnvContainerAPIHostDefault(),
		token:    os.Getenv(envAccessToken),
		c:        http.DefaultClient,
	}, options...)
	// a blank workload is the default workload; egmeta treats a blank workload filter as no filter.
	l.workload = stringsx.FirstNonBlank(l.workload, eg.DefaultWorkload)

	path := egenv.DirectoryCache(eg.AnalyticsCachePath(l.workload))
	if _, err = os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", errorsx.Wrap(err, "unable to check for the cached analytics database")
	}

	id, err := l.find(ctx)
	if err != nil {
		return "", err
	}

	if err = l.download(ctx, id, path); err != nil {
		return "", err
	}

	return path, nil
}

func (t latest) get(ctx context.Context, path string, query url.Values) (*http.Response, error) {
	if stringsx.Blank(t.token) {
		return nil, errorsx.String("access token required")
	}

	uri := fmt.Sprintf("%s%s", t.endpoint, path)
	if len(query) > 0 {
		uri = fmt.Sprintf("%s?%s", uri, query.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to create http request")
	}
	req.Header.Set("Authorization", fmt.Sprintf("BEARER %s", t.token))

	return httpx.AsError(t.c.Do(req))
}

// find the id of the last successful run of the workload.
func (t latest) find(ctx context.Context) (id string, err error) {
	var (
		out struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
	)

	resp, err := t.get(ctx, "/c/history/", url.Values{
		"vcs_uri":  {eggit.EnvCanonicalURI()},
		"workload": {t.workload},
		"result":   {"success"},
		"limit":    {"1"},
	})
	if err != nil {
		errorsx.Log(httpx.AutoClose(resp))
		return "", errorsx.Wrap(err, "unable to search the history")
	}
	defer resp.Body.Close()

	if err = jsonx.UnmarshalRead(resp.Body, &out); err != nil {
		return "", errorsx.Wrap(err, "unable to decode the history")
	}

	if len(out.Items) == 0 {
		return "", errorsx.Wrap(os.ErrNotExist, "no successful runs found")
	}

	return out.Items[0].ID, nil
}

// download the analytics database of the run to path, the destination is replaced atomically.
func (t latest) download(ctx context.Context, id string, path string) (err error) {
	resp, err := t.get(ctx, fmt.Sprintf("/c/history/%s/analytics", url.PathEscape(id)), nil)
	if err != nil {
		errorsx.Log(httpx.AutoClose(resp))
		// free accounts do not have their analytics stored.
		if httpx.IsStatusError(err, http.StatusNotFound, http.StatusPaymentRequired) != nil {
			return errorsx.Wrap(os.ErrNotExist, "analytics database not found")
		}

		return errorsx.Wrap(err, "unable to download the analytics database")
	}
	defer resp.Body.Close()

	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errorsx.Wrap(err, "unable to create the cache directory")
	}

	dst, err := os.CreateTemp(filepath.Dir(path), "download.*.db")
	if err != nil {
		return errorsx.Wrap(err, "unable to create the download file")
	}
	defer os.Remove(dst.Name())
	defer dst.Close()

	if _, err = io.Copy(dst, resp.Body); err != nil {
		return errorsx.Wrap(err, "unable to download the analytics database")
	}

	if err = dst.Close(); err != nil {
		return errorsx.Wrap(err, "unable to write the analytics database")
	}

	return errorsx.Wrap(os.Rename(dst.Name(), path), "unable to cache the analytics database")
}
