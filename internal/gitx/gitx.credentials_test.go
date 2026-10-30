package gitx

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egdaemon/eg/compute"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// serves body as json for every request, recording the requests made.
func jsonclient(t *testing.T, status int, body any, requests *[]*http.Request) *http.Client {
	return &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
		if requests != nil {
			*requests = append(*requests, r)
		}
		w := httptest.NewRecorder()
		w.WriteHeader(status)
		if body != nil {
			require.NoError(t, json.NewEncoder(w).Encode(body))
		}
		return w.Result(), nil
	})}
}

func newpubkey(t *testing.T) ssh.PublicKey {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	k, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	return k
}

func TestRefreshCredentials(t *testing.T) {
	httpcreds := &compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"}

	t.Run("writes the exchanged credentials", func(t *testing.T) {
		var reqs []*http.Request
		dir := t.TempDir()
		require.NoError(t, RefreshCredentials(t.Context(), jsonclient(t, http.StatusOK, httpcreds, &reqs), dir, "token"))
		require.Len(t, reqs, 1)
		require.Equal(t, "BEARER token", reqs[0].Header.Get("Authorization"))
		require.Equal(t, "short-lived", Bearer(dir))
	})

	t.Run("blank token is a noop", func(t *testing.T) {
		var reqs []*http.Request
		dir := t.TempDir()
		require.NoError(t, RefreshCredentials(t.Context(), jsonclient(t, http.StatusOK, httpcreds, &reqs), dir, ""))
		require.Empty(t, reqs)
		require.NoFileExists(t, filepath.Join(dir, "vcsaccess.token"))
	})

	t.Run("failed exchanges error and write nothing", func(t *testing.T) {
		dir := t.TempDir()
		require.Error(t, RefreshCredentials(t.Context(), jsonclient(t, http.StatusUnauthorized, nil, nil), dir, "token"))
		require.NoFileExists(t, filepath.Join(dir, "vcsaccess.token"))
	})

	t.Run("no configured credentials is not a failure and writes nothing", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, RefreshCredentials(t.Context(), jsonclient(t, http.StatusNotFound, nil, nil), dir, "token"))
		require.NoFileExists(t, filepath.Join(dir, "vcsaccess.token"))
	})

	t.Run("unwritable destinations error", func(t *testing.T) {
		require.Error(t, RefreshCredentials(t.Context(), jsonclient(t, http.StatusOK, httpcreds, nil), filepath.Join(t.TempDir(), "missing"), "token"))
	})
}

func TestAutomaticCredentialRefresh(t *testing.T) {
	httpcreds := &compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"}

	t.Run("fetches immediately", func(t *testing.T) {
		ctx, done := context.WithCancel(t.Context())
		defer done()

		dir := t.TempDir()
		require.NoError(t, AutomaticCredentialRefresh(ctx, jsonclient(t, http.StatusOK, httpcreds, nil), dir, "token"))
		require.Equal(t, "short-lived", Bearer(dir))
	})

	t.Run("blank token is skipped", func(t *testing.T) {
		var reqs []*http.Request
		require.NoError(t, AutomaticCredentialRefresh(t.Context(), jsonclient(t, http.StatusOK, httpcreds, &reqs), t.TempDir(), "  "))
		require.Empty(t, reqs)
	})

	t.Run("a failed initial fetch errors", func(t *testing.T) {
		err := AutomaticCredentialRefresh(t.Context(), jsonclient(t, http.StatusUnauthorized, nil, nil), t.TempDir(), "token")
		require.Error(t, err)
	})

	t.Run("no configured credentials continues without refreshing", func(t *testing.T) {
		var reqs []*http.Request
		dir := t.TempDir()

		require.NoError(t, AutomaticCredentialRefresh(t.Context(), jsonclient(t, http.StatusNotFound, nil, &reqs), dir, "token"))
		require.Len(t, reqs, 1)
		require.NoFileExists(t, filepath.Join(dir, "vcsaccess.token"))
		require.Empty(t, Bearer(dir))
	})
}

func TestCredentialExchange(t *testing.T) {
	t.Run("not found means no credentials are configured", func(t *testing.T) {
		_, err := credentialExchange(t.Context(), jsonclient(t, http.StatusNotFound, nil, nil), "token")
		require.ErrorIs(t, err, ErrNoCredentials)
	})

	t.Run("unauthorized is a failure and not a lack of credentials", func(t *testing.T) {
		_, err := credentialExchange(t.Context(), jsonclient(t, http.StatusUnauthorized, nil, nil), "token")
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrNoCredentials)
	})
}

func TestLoadCredentials(t *testing.T) {
	write := func(t *testing.T, v any) string {
		dir := t.TempDir()
		encoded, err := json.Marshal(v)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "vcsaccess.token"), encoded, 0600))
		return dir
	}

	t.Run("http uris receive credentials", func(t *testing.T) {
		dir := write(t, &compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"})
		opt, err := LoadCredentials(t.Context(), "https://github.com/example/repo.git", dir)
		require.NoError(t, err)
		require.NotNil(t, opt)
	})

	t.Run("non-http uris receive none", func(t *testing.T) {
		dir := write(t, &compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"})
		opt, err := LoadCredentials(t.Context(), "git@github.com:example/repo.git", dir)
		require.NoError(t, err)
		require.Nil(t, opt)
	})

	t.Run("incomplete credentials receive none", func(t *testing.T) {
		dir := write(t, &compute.GitCredentialsHTTP{Username: "git"})
		opt, err := LoadCredentials(t.Context(), "https://github.com/example/repo.git", dir)
		require.NoError(t, err)
		require.Nil(t, opt)
	})

	t.Run("a missing token file errors", func(t *testing.T) {
		_, err := LoadCredentials(t.Context(), "https://github.com/example/repo.git", t.TempDir())
		require.Error(t, err)
	})
}

func TestSSHCredentials(t *testing.T) {
	const sshuri = "git@github.com:example/repo.git"
	pub := newpubkey(t)
	encode := func(t *testing.T, v any) []byte {
		b, err := json.Marshal(v)
		require.NoError(t, err)
		return b
	}

	t.Run("ssh uris receive a remote signer", func(t *testing.T) {
		encoded := encode(t, &compute.GitCredentialsSSH{Username: "git", PublicKey: pub.Marshal()})
		require.NotNil(t, sshcredentials(t.Context(), http.DefaultClient, sshuri, "token", encoded))
	})

	t.Run("http uris receive none", func(t *testing.T) {
		encoded := encode(t, &compute.GitCredentialsSSH{Username: "git", PublicKey: pub.Marshal()})
		require.Nil(t, sshcredentials(t.Context(), http.DefaultClient, "https://github.com/example/repo.git", "token", encoded))
	})

	t.Run("unusable credentials receive none", func(t *testing.T) {
		require.Nil(t, sshcredentials(t.Context(), http.DefaultClient, sshuri, "token", []byte("not json")))
		require.Nil(t, sshcredentials(t.Context(), http.DefaultClient, sshuri, "token", encode(t, &compute.GitCredentialsSSH{Username: "git"})))
		require.Nil(t, sshcredentials(t.Context(), http.DefaultClient, sshuri, "token", encode(t, &compute.GitCredentialsSSH{Username: "git", PublicKey: []byte("junk")})))
	})

	t.Run("exchange selects ssh credentials for ssh uris", func(t *testing.T) {
		c := jsonclient(t, http.StatusOK, &compute.GitCredentialsSSH{Username: "git", PublicKey: pub.Marshal()}, nil)
		opt, err := ExchangeCredentials(t.Context(), c, sshuri, "token")
		require.NoError(t, err)
		require.NotNil(t, opt)
	})
}

func TestRemoteSigner(t *testing.T) {
	pub := newpubkey(t)

	t.Run("delegates signing to the credential service", func(t *testing.T) {
		var reqs []*http.Request
		var got compute.GitSignRequest
		c := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			reqs = append(reqs, r)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			w := httptest.NewRecorder()
			require.NoError(t, json.NewEncoder(w).Encode(&compute.GitSignatureSSH{Format: "ssh-ed25519", Blob: []byte("signed")}))
			return w.Result(), nil
		})}

		signer := remotesigner{ctx: t.Context(), c: c, token: "token", pubkey: pub}
		require.Equal(t, pub, signer.PublicKey())

		sig, err := signer.SignWithAlgorithm(nil, []byte("payload"), "rsa-sha2-512")
		require.NoError(t, err)
		require.Equal(t, &ssh.Signature{Format: "ssh-ed25519", Blob: []byte("signed")}, sig)

		require.Len(t, reqs, 1)
		require.Equal(t, http.MethodPost, reqs[0].Method)
		require.True(t, strings.HasSuffix(reqs[0].URL.Path, "/r/vcsaccess/sign"), reqs[0].URL.Path)
		require.Equal(t, "BEARER token", reqs[0].Header.Get("Authorization"))
		require.Equal(t, "application/json", reqs[0].Header.Get("Content-Type"))
		require.Equal(t, []byte("payload"), got.Data)
		require.Equal(t, "rsa-sha2-512", got.Algorithm)
	})

	t.Run("sign uses the default algorithm", func(t *testing.T) {
		var got compute.GitSignRequest
		c := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			w := httptest.NewRecorder()
			require.NoError(t, json.NewEncoder(w).Encode(&compute.GitSignatureSSH{Format: "ssh-ed25519", Blob: []byte("signed")}))
			return w.Result(), nil
		})}

		_, err := remotesigner{ctx: t.Context(), c: c, token: "token", pubkey: pub}.Sign(nil, []byte("payload"))
		require.NoError(t, err)
		require.Empty(t, got.Algorithm)
	})

	t.Run("service failures error", func(t *testing.T) {
		_, err := remotesigner{ctx: t.Context(), c: jsonclient(t, http.StatusForbidden, nil, nil), token: "token", pubkey: pub}.Sign(nil, []byte("payload"))
		require.Error(t, err)
	})

	t.Run("undecodable signatures error", func(t *testing.T) {
		c := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			_, _ = w.WriteString("not json")
			return w.Result(), nil
		})}
		_, err := remotesigner{ctx: t.Context(), c: c, token: "token", pubkey: pub}.Sign(nil, []byte("payload"))
		require.Error(t, err)
	})
}
