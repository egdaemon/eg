package egiso

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
)

// Remote reads the archive at the URL with range requests, so only the parts that are read are
// fetched.
func Remote(ctx context.Context, c *http.Client, url string) (_ *io.SectionReader, err error) {
	var (
		req  *http.Request
		resp *http.Response
	)

	if req, err = http.NewRequestWithContext(ctx, http.MethodHead, url, nil); err != nil {
		return nil, err
	}
	if resp, err = httpx.AsError(c.Do(req)); err != nil {
		return nil, errorsx.Wrapf(err, "unable to find the size of %s", url)
	}
	defer httpx.AutoClose(resp)

	if resp.ContentLength < 0 {
		return nil, errorsx.Errorf("%s does not report its size", url)
	}

	return io.NewSectionReader(ranges{ctx: ctx, c: c, url: url}, 0, resp.ContentLength), nil
}

// ranges reads a URL with range requests. io.ReaderAt takes no context, so the requests use the
// one the archive was opened with.
type ranges struct {
	ctx context.Context
	c   *http.Client
	url string
}

func (t ranges) ReadAt(p []byte, off int64) (n int, err error) {
	var (
		req  *http.Request
		resp *http.Response
	)

	if len(p) == 0 {
		return 0, nil
	}

	if req, err = httpx.Get(t.ctx, t.url); err != nil {
		return 0, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+int64(len(p))-1))

	if resp, err = httpx.AsError(t.c.Do(req)); err != nil {
		return 0, err
	}
	defer httpx.AutoClose(resp)

	// a server that ignores the range sends the whole archive, which would be read from its start.
	if resp.StatusCode != http.StatusPartialContent {
		return 0, errorsx.Errorf("%s does not serve ranges: %s", t.url, resp.Status)
	}

	return io.ReadFull(resp.Body, p)
}
