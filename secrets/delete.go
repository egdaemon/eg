package secrets

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"google.golang.org/api/iterator"
)

// Delete makes the secret referenced by uri unreadable. for gcpsm every enabled version is disabled, for awssm
// (which cannot disable a secret) it is scheduled for deletion using the account's default recovery window,
// during which it cannot be read and can be restored. the file schemes are removed, and a missing file is an
// error that matches os.ErrNotExist, use Exists to check beforehand.
func Delete(ctx context.Context, uri string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return err
	}

	switch u.Scheme {
	case "gcpsm":
		return deleteGCP(ctx, u)
	case "awssm":
		return deleteAWS(ctx, u)
	case "chachasm":
		return os.Remove(chachaPath(u))
	case "file":
		return os.Remove(filePath(u))
	default:
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
}

func deleteGCP(ctx context.Context, u *url.URL) error {
	secretName := strings.Split(strings.Trim(u.Path, "/"), "/")[0]
	if secretName == "" {
		return fmt.Errorf("missing secret name in path")
	}

	client, err := newgcpclient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	// gcp has no way to disable a secret as a whole, so every enabled version is disabled instead.
	// disabled versions can be re-enabled, the secret and its history are kept.
	versions := client.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
		Parent: fmt.Sprintf("projects/%s/secrets/%s", u.Host, secretName),
		Filter: "state:ENABLED",
	})
	for {
		v, err := versions.Next()
		if errors.Is(err, iterator.Done) {
			return nil
		}
		if err != nil {
			return err
		}

		if _, err = client.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{Name: v.GetName()}); err != nil {
			return err
		}
	}
}

func deleteAWS(ctx context.Context, u *url.URL) error {
	secretID := strings.Split(strings.Trim(u.Path, "/"), "/")[0]
	if secretID == "" {
		return fmt.Errorf("missing secret name in path")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(u.Query().Get("region")))
	if err != nil {
		return err
	}

	_, err = secretsmanager.NewFromConfig(cfg).DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId: aws.String(secretID),
	})
	return err
}
