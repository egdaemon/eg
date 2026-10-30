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
	awstypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Exists reports whether the secret referenced by uri already exists, without reading its value:
// the hosted secret managers are asked for the secret's metadata, and the file schemes are only
// stat'ed, so no passphrase is needed for chachasm.
func Exists(ctx context.Context, uri string) (bool, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return false, err
	}

	switch u.Scheme {
	case "gcpsm":
		return existsGCP(ctx, u)
	case "awssm":
		return existsAWS(ctx, u)
	case "chachasm":
		return existsFile(chachaPath(u))
	case "file":
		return existsFile(filePath(u))
	default:
		return false, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
}

// chachaPath is the file a chachasm uri refers to, see downloadCHACHA and updateCHACHA.
func chachaPath(u *url.URL) string {
	if u.Host != "" && !strings.Contains(u.Host, "@") {
		return u.Host + u.Path
	}

	return u.Path
}

func existsFile(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err
}

func existsGCP(ctx context.Context, u *url.URL) (bool, error) {
	secretName := strings.Split(strings.Trim(u.Path, "/"), "/")[0]
	if secretName == "" {
		return false, fmt.Errorf("missing secret name in path")
	}

	client, err := newgcpclient(ctx)
	if err != nil {
		return false, err
	}
	defer client.Close()

	_, err = client.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s", u.Host, secretName),
	})
	if err == nil {
		return true, nil
	}

	if s, ok := status.FromError(err); ok && s.Code() == codes.NotFound {
		return false, nil
	}

	return false, err
}

func existsAWS(ctx context.Context, u *url.URL) (bool, error) {
	secretID := strings.Split(strings.Trim(u.Path, "/"), "/")[0]
	if secretID == "" {
		return false, fmt.Errorf("missing secret name in path")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(u.Query().Get("region")))
	if err != nil {
		return false, err
	}

	_, err = secretsmanager.NewFromConfig(cfg).DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(secretID),
	})
	if err == nil {
		return true, nil
	}

	var notFound *awstypes.ResourceNotFoundException
	if errors.As(err, &notFound) {
		return false, nil
	}

	return false, err
}
