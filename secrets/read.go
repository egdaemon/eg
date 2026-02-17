package secrets

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

type readOptions struct {
	passphrase string
}

type ReadOption func(*readOptions)

func WithPassphrase(p string) ReadOption {
	return func(o *readOptions) {
		o.passphrase = p
	}
}

// NewReader returns an io.Reader that streams the secrets for each URI,
// separated by newlines. Each secret is read lazily on demand.
func NewReader(ctx context.Context, uris ...string) io.Reader {
	var suffix []byte
	if len(uris) > 1 {
		suffix = []byte("\n")
	}
	r := &secretsReader{ctx: ctx, uris: uris, suffix: suffix}
	return r
}

type secretsReader struct {
	ctx     context.Context
	uris    []string
	suffix  []byte
	idx     int
	current io.Reader
}

func (t *secretsReader) Read(p []byte) (int, error) {
	for {
		if t.current != nil {
			n, err := t.current.Read(p)
			if err == io.EOF {
				t.current = nil
				continue
			}
			return n, err
		}

		if t.idx >= len(t.uris) {
			return 0, io.EOF
		}

		uri := t.uris[t.idx]
		t.idx++
		t.current = io.MultiReader(Read(t.ctx, uri), bytes.NewReader(t.suffix))
		log.Println("Reading", uri)
	}
}

func Read(ctx context.Context, uri string, options ...ReadOption) io.Reader {
	opts := &readOptions{}
	for _, o := range options {
		o(opts)
	}

	u, err := url.Parse(uri)
	if err != nil {
		return errorsx.Reader(err)
	}

	// Extract passphrase from URI userinfo if not already set via options
	opts.passphrase = langx.FirstNonZero(opts.passphrase, u.User.Username())

	switch u.Scheme {
	case "gcpsm":
		return downloadGCP(ctx, u, opts)
	case "awssm":
		return downloadAWS(ctx, u, opts)
	case "chachasm":
		return downloadCHACHA(ctx, u, opts)
	case "file":
		return downloadFile(ctx, u)
	default:
		return errorsx.Reader(fmt.Errorf("unsupported scheme: %s", u.Scheme))
	}
}

func downloadGCP(ctx context.Context, u *url.URL, opts *readOptions) io.Reader {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return errorsx.Reader(err)
	}
	defer client.Close()

	projectID := u.Host
	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) < 1 {
		return errorsx.Reader(fmt.Errorf("missing secret name in path"))
	}

	secretName := pathParts[0]
	version := "latest"
	if len(pathParts) > 1 {
		version = pathParts[1]
	}

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/%s", projectID, secretName, version),
	}

	resp, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return errorsx.Reader(err)
	}

	return bytes.NewReader(resp.Payload.Data)
}

func downloadAWS(ctx context.Context, u *url.URL, opts *readOptions) io.Reader {
	region := u.Query().Get("region")
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return errorsx.Reader(err)
	}

	client := secretsmanager.NewFromConfig(cfg)
	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) < 1 {
		return errorsx.Reader(fmt.Errorf("missing secret name in path"))
	}

	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(pathParts[0]),
	}

	if len(pathParts) > 1 {
		version := pathParts[1]
		if len(version) > 16 {
			input.VersionId = aws.String(version)
		} else {
			input.VersionStage = aws.String(version)
		}
	}

	resp, err := client.GetSecretValue(ctx, input)
	if err != nil {
		return errorsx.Reader(err)
	}

	if resp.SecretString != nil {
		return strings.NewReader(*resp.SecretString)
	}

	if resp.SecretBinary != nil {
		return bytes.NewReader(resp.SecretBinary)
	}

	return errorsx.Reader(fmt.Errorf("secret contains no data"))
}

// filePath resolves the file path from a file:// URI.
// file:///absolute/path -> u.Path = "/absolute/path"
// file:./relative/path  -> u.Opaque = "./relative/path"
func filePath(u *url.URL) string {
	if u.Opaque != "" {
		return u.Opaque
	}
	return u.Path
}

func downloadFile(ctx context.Context, u *url.URL) io.Reader {
	data, err := os.ReadFile(filePath(u))
	if err != nil {
		return errorsx.Reader(err)
	}
	return bytes.NewReader(data)
}

func downloadCHACHA(ctx context.Context, u *url.URL, opts *readOptions) io.Reader {
	if opts.passphrase == "" {
		return errorsx.Reader(fmt.Errorf("passphrase not provided"))
	}

	filePath := u.Path
	if u.Host != "" && !strings.Contains(u.Host, "@") {
		filePath = u.Host + u.Path
	}

	ciphertext, err := os.ReadFile(filePath)
	if err != nil {
		return errorsx.Reader(err)
	}

	// Structure: [SALT(16 bytes)][NONCE(12 bytes)][ENCRYPTED_DATA]
	if len(ciphertext) < 16+12 {
		return errorsx.Reader(fmt.Errorf("invalid ciphertext format"))
	}

	salt := ciphertext[:16]
	nonce := ciphertext[16:28]
	data := ciphertext[28:]

	// Derive key using Argon2id
	key := argon2.IDKey([]byte(opts.passphrase), salt, 1, 64*1024, 4, 32)

	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return errorsx.Reader(err)
	}

	plaintext, err := aead.Open(nil, nonce, data, nil)
	if err != nil {
		return errorsx.Reader(fmt.Errorf("decryption failed: %w", err))
	}

	return bytes.NewReader(plaintext)
}
