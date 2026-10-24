package secrets

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/stretchr/testify/require"
)

func TestAWSConfig(t *testing.T) {
	// an sts endpoint that answers AssumeRoleWithWebIdentity and records what it was asked.
	sts := func(t *testing.T, form *http.Request) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, r.ParseForm())
			*form = *r
			w.Header().Set("Content-Type", "text/xml")
			fmt.Fprint(w, `<AssumeRoleWithWebIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><AssumeRoleWithWebIdentityResult><Credentials><AccessKeyId>akid</AccessKeyId><SecretAccessKey>secret</SecretAccessKey><SessionToken>session</SessionToken><Expiration>2099-01-01T00:00:00Z</Expiration></Credentials></AssumeRoleWithWebIdentityResult></AssumeRoleWithWebIdentityResponse>`)
		}))
		t.Cleanup(srv.Close)
		return srv
	}

	t.Run("should assume the role with the identity token of the workload", func(t *testing.T) {
		var asked http.Request
		srv := sts(t, &asked)

		tokenfile := filepath.Join(t.TempDir(), "token")
		require.NoError(t, os.WriteFile(tokenfile, []byte("idtoken"), 0600))

		ctx := Env().AWS("arn:aws:iam::123456789012:role/example", tokenfile, "us-east-2", "eg-1").context(t.Context())
		cfg, err := awsconfig(ctx, "", config.WithBaseEndpoint(srv.URL))
		require.NoError(t, err)
		require.Equal(t, "us-east-2", cfg.Region, "the region of the connection is the default")

		creds, err := cfg.Credentials.Retrieve(ctx)
		require.NoError(t, err)
		require.Equal(t, "akid", creds.AccessKeyID)
		require.Equal(t, "arn:aws:iam::123456789012:role/example", asked.Form.Get("RoleArn"))
		require.Equal(t, "idtoken", asked.Form.Get("WebIdentityToken"))
		require.Equal(t, "eg-1", asked.Form.Get("RoleSessionName"))
	})

	t.Run("should prefer the region of the secret", func(t *testing.T) {
		ctx := Env().AWS("arn:aws:iam::123456789012:role/example", "/token", "us-east-2", "eg-1").context(t.Context())
		cfg, err := awsconfig(ctx, "eu-west-1")
		require.NoError(t, err)
		require.Equal(t, "eu-west-1", cfg.Region)
	})

	t.Run("should use the ambient credentials without an identity", func(t *testing.T) {
		t.Setenv("AWS_ACCESS_KEY_ID", "ambient")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")

		cfg, err := awsconfig(t.Context(), "us-east-1")
		require.NoError(t, err)

		creds, err := cfg.Credentials.Retrieve(t.Context())
		require.NoError(t, err)
		require.Equal(t, "ambient", creds.AccessKeyID)
	})

	t.Run("should leave the reader unchanged without a role", func(t *testing.T) {
		ctx := Env().AWS("", "", "", "").context(t.Context())
		require.Nil(t, ctx.Value(awsidentitykey{}))
	})
}
