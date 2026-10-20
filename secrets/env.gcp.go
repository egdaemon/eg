package secrets

import (
	"context"

	"google.golang.org/api/option"
)

type gcpcredentialskey struct{}

// gcpcredentials are the client options injected into the context, none when there are none so the
// clients use the ambient credentials.
func gcpcredentials(ctx context.Context) []option.ClientOption {
	options, _ := ctx.Value(gcpcredentialskey{}).([]option.ClientOption)
	return options
}

// GCP reads gcpsm secrets with the credentials at the given path, an external_account application default
// credentials file, instead of the ambient ones. they apply to the reads of this reader only. a blank path
// leaves the credentials unchanged.
//
//	secrets.Env().GCP(path).NewReader(ctx, uris...)
func (t *EnvReader) GCP(adc string) *EnvReader {
	if adc != "" {
		// the credentials of the clients only, the process environment is shared.
		t.gcp = append(t.gcp, option.WithAuthCredentialsFile(option.ExternalAccount, adc))
	}

	return t
}

func (t *EnvReader) gcpcontext(ctx context.Context) context.Context {
	if len(t.gcp) > 0 {
		ctx = context.WithValue(ctx, gcpcredentialskey{}, t.gcp)
	}

	return ctx
}
