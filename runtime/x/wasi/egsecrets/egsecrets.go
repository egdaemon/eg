package egsecrets

import (
	"context"
	"io"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/secrets"
)

// NewReader returns an io.Reader that streams the secrets for each URI,
// separated by newlines. Each secret is read lazily on demand.
func NewReader(ctx context.Context, uris ...string) io.Reader {
	return secrets.NewReader(ctx, uris...)
}

// Read a secret from the given URI. Supported schemes: gcpsm://, awssm://, chachasm://, file://
func Read(ctx context.Context, uri string) io.Reader {
	return secrets.Read(ctx, uri)
}

// Update a secret at the given URI. Supported schemes: gcpsm://, awssm://, chachasm://, file://
func Update(ctx context.Context, uri string, r io.Reader) error {
	return secrets.Update(ctx, uri, r)
}

// Env reads secrets from the given URIs and parses them as .env formatted data
// (one KEY=VALUE per line), returning the resulting environment variables.
func Env(ctx context.Context, uris ...string) []string {
	return errorsx.Must(envx.FromReader(NewReader(ctx, uris...)))
}
