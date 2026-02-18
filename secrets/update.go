package secrets

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	awstypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/egdaemon/eg/internal/langx"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Update(ctx context.Context, uri string, r io.Reader, options ...ReadOption) error {
	opts := &readOptions{}
	for _, o := range options {
		o(opts)
	}

	u, err := url.Parse(uri)
	if err != nil {
		return err
	}

	opts.passphrase = langx.FirstNonZero(opts.passphrase, u.User.Username())

	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	switch u.Scheme {
	case "gcpsm":
		return updateGCP(ctx, u, data)
	case "awssm":
		return updateAWS(ctx, u, data)
	case "chachasm":
		return updateCHACHA(u, data, opts)
	case "file":
		return os.WriteFile(filePath(u), data, 0600)
	default:
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
}

func updateGCP(ctx context.Context, u *url.URL, data []byte) error {
	client, err := newgcpclient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	project := u.Host
	secretName := strings.Split(strings.Trim(u.Path, "/"), "/")[0]
	secretPath := fmt.Sprintf("projects/%s/secrets/%s", project, secretName)

	_, err = client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretPath,
		Payload: &secretmanagerpb.SecretPayload{
			Data: data,
		},
	})
	if err == nil {
		return nil
	}

	if s, ok := status.FromError(err); !ok || s.Code() != codes.NotFound {
		return err
	}

	// Secret doesn't exist yet; create it then add the first version.
	if _, err = client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   fmt.Sprintf("projects/%s", project),
		SecretId: secretName,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	}); err != nil {
		return err
	}

	_, err = client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretPath,
		Payload: &secretmanagerpb.SecretPayload{
			Data: data,
		},
	})
	return err
}

func updateAWS(ctx context.Context, u *url.URL, data []byte) error {
	region := u.Query().Get("region")
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return err
	}

	client := secretsmanager.NewFromConfig(cfg)
	secretID := strings.Split(strings.Trim(u.Path, "/"), "/")[0]

	_, err = client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secretID),
		SecretBinary: data,
	})
	if err == nil {
		return nil
	}

	var notFound *awstypes.ResourceNotFoundException
	if !errors.As(err, &notFound) {
		return err
	}

	// Secret doesn't exist yet; create it with the initial value.
	_, err = client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretID),
		SecretBinary: data,
	})
	return err
}

func updateCHACHA(u *url.URL, data []byte, opts *readOptions) error {
	if opts.passphrase == "" {
		return fmt.Errorf("passphrase required for encryption")
	}

	filePath := u.Path
	if u.Host != "" && !strings.Contains(u.Host, "@") {
		filePath = u.Host + u.Path
	}

	salt := make([]byte, 16)
	nonce := make([]byte, chacha20poly1305.NonceSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	key := argon2.IDKey([]byte(opts.passphrase), salt, 1, 64*1024, 4, 32)
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return err
	}

	ciphertext := aead.Seal(nil, nonce, data, nil)

	// Construct: Salt + Nonce + Ciphertext
	var final bytes.Buffer
	final.Write(salt)
	final.Write(nonce)
	final.Write(ciphertext)

	return os.WriteFile(filePath, final.Bytes(), 0600)
}
