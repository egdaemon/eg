package secrets

import (
	"context"
	"io"

	"google.golang.org/api/option"
)

// EnvReader reads secrets with credentials injected into the lookup of the secret managers.
type EnvReader struct {
	gcp []option.ClientOption
}

// Env builds a reader whose secrets are read with the given credentials, see GCP.
func Env() *EnvReader {
	return &EnvReader{}
}

// NewReader returns a reader of the secrets for each URI, see the package level NewReader.
func (t *EnvReader) NewReader(ctx context.Context, uris ...string) io.Reader {
	return NewReader(t.context(ctx), uris...)
}

// context injects the credentials into the lookups made with the returned context.
func (t *EnvReader) context(ctx context.Context) context.Context {
	return t.gcpcontext(ctx)
}
