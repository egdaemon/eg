package runners

import (
	"io"
	"mime/multipart"
	"os"
	"strconv"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
)

func NewEnqueueUpload(enq *Enqueued, archive io.Reader) (mimetype string, body *os.File, err error) {
	return httpx.Multipart(func(w *multipart.Writer) error {
		if err = w.WriteField("entry", enq.Entry); err != nil {
			return errorsx.Wrap(err, "unable to copy entry point")
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
			return errorsx.Wrap(err, "unable to isa architecture")
		}

		if err = w.WriteField("os", enq.Os); err != nil {
			return errorsx.Wrap(err, "unable to operating system")
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

func NewEnqueueCompletion(logs io.Reader) (mimetype string, body *os.File, err error) {
	return httpx.Multipart(func(w *multipart.Writer) error {
		part, lerr := w.CreatePart(httpx.NewMultipartHeader("text/plain", "logs", "daemon.logs"))
		if lerr != nil {
			return errorsx.Wrap(lerr, "unable to create logs part")
		}

		if _, lerr = io.Copy(part, logs); lerr != nil {
			return errorsx.Wrap(lerr, "unable to copy logs")
		}

		return nil
	})
}
