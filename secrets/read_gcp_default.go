//go:build !(wasip1 && wasm)

package secrets

import (
	"context"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
)

func newgcpclient(ctx context.Context) (*secretmanager.Client, error) {
	return secretmanager.NewClient(ctx)
}
