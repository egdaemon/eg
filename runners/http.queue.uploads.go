package runners

import (
	"encoding/json"
	"io"
	"mime/multipart"
	"strconv"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
)

// NewWorkloadRequest builds the multipart body for POST /c/enqueue: an
// "enqueued" field carrying the JSON-encoded EnqueuedDequeueResponse -- the
// same shape a runner gets back from the polling /c/q/dequeue flow, whose
// Enqueued.VcsCommit is the treeish to check out and whose AccessToken is
// the short-lived token to exchange for actual git credentials -- and a
// required "environ" file part (see eg.EnvironFile), only used as a fallback
// for callers that predate those structured fields. Pass an empty reader
// when neither applies; the part itself must still be present.
func NewWorkloadRequest(resp *EnqueuedDequeueResponse, environ io.Reader) (mimetype string, body io.ReadCloser, err error) {
	return httpx.Multipart(func(w *multipart.Writer) error {
		encoded, lerr := json.Marshal(resp)
		if lerr != nil {
			return errorsx.Wrap(lerr, "unable to encode enqueue request")
		}

		if lerr = w.WriteField("enqueued", string(encoded)); lerr != nil {
			return errorsx.Wrap(lerr, "unable to copy enqueued metadata")
		}

		part, lerr := w.CreatePart(httpx.NewMultipartHeader("text/plain", "environ", eg.EnvironFile))
		if lerr != nil {
			return errorsx.Wrap(lerr, "unable to create environ part")
		}

		if _, lerr = io.Copy(part, environ); lerr != nil {
			return errorsx.Wrap(lerr, "unable to copy environ")
		}

		return nil
	})
}

func NewEnqueueUpload(enq *Enqueued, archive io.Reader) (mimetype string, body io.ReadCloser, err error) {
	return httpx.Multipart(func(w *multipart.Writer) error {
		if err = w.WriteField("entry", enq.Entry); err != nil {
			return errorsx.Wrap(err, "unable to copy entry point")
		}

		if err = w.WriteField("allow_shared", strconv.FormatBool(enq.AllowShared)); err != nil {
			return errorsx.Wrap(err, "unable to copy allow_shared")
		}

		if err = w.WriteField("ttl", strconv.FormatUint(enq.Ttl, 10)); err != nil {
			return errorsx.Wrap(err, "unable to set ttl")
		}

		if err = w.WriteField("cores", strconv.FormatUint(enq.Cores, 10)); err != nil {
			return errorsx.Wrap(err, "unable to set minimum cores")
		}

		if err = w.WriteField("memory", strconv.FormatUint(enq.Memory, 10)); err != nil {
			return errorsx.Wrap(err, "unable to set minimum memory")
		}

		if err = w.WriteField("arch", enq.Arch); err != nil {
			return errorsx.Wrap(err, "unable to set isa architecture")
		}

		if err = w.WriteField("os", enq.Os); err != nil {
			return errorsx.Wrap(err, "unable to set operating system")
		}

		if err = w.WriteField("vcs_uri", enq.VcsUri); err != nil {
			return errorsx.Wrap(err, "unable to set vcsuri")
		}

		if err = w.WriteField("description", enq.Description); err != nil {
			return errorsx.Wrap(err, "unable to set description")
		}

		part, lerr := w.CreatePart(httpx.NewMultipartHeader("application/gzip", "archive", "archive.tar.gz"))
		if lerr != nil {
			return errorsx.Wrap(lerr, "unable to create archive part")
		}

		if _, lerr = io.Copy(part, archive); lerr != nil {
			return errorsx.Wrap(lerr, "unable to copy archive")
		}

		return nil
	})
}

func NewEnqueueCompletion(cause error, duration time.Duration, logs io.Reader, analytics io.Reader) (mimetype string, body io.ReadCloser, err error) {
	return httpx.Multipart(func(w *multipart.Writer) error {
		if err = w.WriteField("duration", strconv.FormatUint(uint64(duration.Milliseconds()), 10)); err != nil {
			return errorsx.Wrap(err, "unable to write duration")
		}

		if err = w.WriteField("successful", strconv.FormatBool(cause == nil)); err != nil {
			return errorsx.Wrap(err, "unable to write completion state")
		}

		part, lerr := w.CreatePart(httpx.NewMultipartHeader("text/plain", "logs", "daemon.logs"))
		if lerr != nil {
			return errorsx.Wrap(lerr, "unable to create logs part")
		}

		if _, lerr = io.Copy(part, logs); lerr != nil {
			return errorsx.Wrap(lerr, "unable to copy logs")
		}

		apart, aerr := w.CreatePart(httpx.NewMultipartHeader("application/vnd.egdaemon-analytics", "analytics", "analytics.db"))
		if aerr != nil {
			return errorsx.Wrap(aerr, "unable to create analytics part")
		}

		if _, lerr = io.Copy(apart, analytics); lerr != nil {
			return errorsx.Wrap(lerr, "unable to copy analytics")
		}

		return nil
	})
}
