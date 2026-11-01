// Package egtemplate provides helpers for rendering text templates.
package egtemplate

import (
	"bytes"
	"io"
	"sync"
)

// Executor is the rendering behavior of a template, satisfied by *text/template.Template
// and *html/template.Template.
type Executor interface {
	Execute(w io.Writer, data any) error
}

// Reader returns a reader of the template rendered with the given data. rendering is deferred until the
// first Read, so values the template computes can depend on work completed after the reader was created.
// rendering errors are reported by Read, which lets the result be handed to anything that consumes
// an io.Reader (e.g. egfs.Install) without partial output.
func Reader(t Executor, data any) io.Reader {
	return &lazy{t: t, data: data}
}

type lazy struct {
	t        Executor
	data     any
	once     sync.Once
	rendered bytes.Buffer
	err      error
}

func (t *lazy) Read(p []byte) (int, error) {
	t.once.Do(func() {
		t.err = t.t.Execute(&t.rendered, t.data)
	})

	if t.err != nil {
		return 0, t.err
	}

	return t.rendered.Read(p)
}
