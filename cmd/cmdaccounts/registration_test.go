package cmdaccounts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/httptestx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func TestSignupRun(t *testing.T) {
	t.Run("account id with an existing profile skips selection and logs in", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"account_id": "account-1"}).SignedString([]byte("secret"))
		require.NoError(t, err)
		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: signed})
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			switch {
			case req.Method == http.MethodGet && req.URL.Path == "/authn/s/":
				require.Equal(t, "account-1", req.URL.Query().Get("account_id"))
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(options)), Header: http.Header{}}
			case req.Method == http.MethodPost && req.URL.Path == "/authn/ssh":
				encoded, err := json.Marshal(authn.Authed{
					Profiles: []*authn.Authn{
						{Token: "the-token", Profile: &authn.Profile{Id: "profile-1", Display: "dave", AccountId: "account-1"}},
					},
				})
				require.NoError(t, err)
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil
			}
		})

		var sessions int
		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			// neither signup nor register may be requested when a profile already exists.
			require.Equal(t, "/authn/current", req.URL.Path)
			require.Equal(t, "BEARER the-token", req.Header.Get("Authorization"))
			sessions++
			encoded, err := json.Marshal(authn.Current{
				Profile: &authn.Profile{Id: "profile-1", Display: "dave"},
				Account: &authn.Account{Id: "account-1", Display: "dave's account"},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		err = Signup{SSHKeyPath: "/path/to/key"}.run(context.Background(), &cmdopts.Global{AccountID: "account-1", Context: context.Background()}, oauthc, chttp, signer, func(*huh.Select[*authn.Authn], ...huh.Option[*authn.Authn]) (*authn.Authn, error) {
			t.Fatal("unexpected picker invocation")
			return nil, nil
		})
		require.NoError(t, err)
		require.Equal(t, 1, sessions)
	})

	t.Run("no profiles and no sso domain signs up for a new account", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": ""}).SignedString([]byte("secret"))
		require.NoError(t, err)
		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: signed})
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			switch req.URL.Path {
			case "/authn/s/":
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(options)), Header: http.Header{}}
			case "/authn/ssh":
				encoded, err := json.Marshal(authn.Authed{})
				require.NoError(t, err)
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil
			}
		})

		var signups int
		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, "/authn/signup", req.URL.Path)

			// the ssh signing alg is unknown to the jwt parser, read the payload segment directly.
			segments := strings.Split(strings.TrimPrefix(req.Header.Get("Authorization"), "BEARER "), ".")
			require.Len(t, segments, 3)
			payload, err := base64.RawURLEncoding.DecodeString(segments[1])
			require.NoError(t, err)
			var claims jwt.RegisteredClaims
			require.NoError(t, json.Unmarshal(payload, &claims))
			require.Empty(t, claims.Issuer)

			signups++
			encoded, err := json.Marshal(authn.Current{
				Profile: &authn.Profile{Id: "profile-1", Display: "dave"},
				Account: &authn.Account{Id: "account-1", Display: "dave's account"},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		err = Signup{SSHKeyPath: "/path/to/key"}.run(context.Background(), &cmdopts.Global{Context: context.Background()}, oauthc, chttp, signer, func(*huh.Select[*authn.Authn], ...huh.Option[*authn.Authn]) (*authn.Authn, error) {
			t.Fatal("unexpected picker invocation")
			return nil, nil
		})
		require.NoError(t, err)
		require.Equal(t, 1, signups)
	})

	t.Run("no profiles with an sso domain requests access to the account", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": "sso-1"}).SignedString([]byte("secret"))
		require.NoError(t, err)
		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: signed})
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			switch req.URL.Path {
			case "/authn/s/":
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(options)), Header: http.Header{}}
			case "/authn/ssh":
				encoded, err := json.Marshal(authn.Authed{})
				require.NoError(t, err)
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil
			}
		})

		var registrations int
		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, "/authn/register", req.URL.Path)

			// the ssh signing alg is unknown to the jwt parser, read the payload segment directly.
			segments := strings.Split(strings.TrimPrefix(req.Header.Get("Authorization"), "BEARER "), ".")
			require.Len(t, segments, 3)
			payload, err := base64.RawURLEncoding.DecodeString(segments[1])
			require.NoError(t, err)
			var claims jwt.RegisteredClaims
			require.NoError(t, json.Unmarshal(payload, &claims))
			require.Equal(t, "sso-1", claims.Issuer)

			registrations++
			encoded, err := json.Marshal(authn.Current{
				Profile: &authn.Profile{Id: "profile-1", Display: "dave"},
				Account: &authn.Account{Id: "account-1", Display: "dave's account"},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		err = Signup{SSHKeyPath: "/path/to/key", Account: "acme", Endpoint: "https://example.invalid"}.run(context.Background(), &cmdopts.Global{Context: context.Background()}, oauthc, chttp, signer, func(*huh.Select[*authn.Authn], ...huh.Option[*authn.Authn]) (*authn.Authn, error) {
			t.Fatal("unexpected picker invocation")
			return nil, nil
		})
		require.NoError(t, err)
		require.Equal(t, 1, registrations)
	})

	t.Run("login options failure is reported", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			require.Equal(t, "/authn/s/", req.URL.Path)
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewReader(nil)), Header: http.Header{}}
		})
		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil
		})

		err = Signup{SSHKeyPath: "/path/to/key"}.run(context.Background(), &cmdopts.Global{Context: context.Background()}, oauthc, chttp, signer, func(*huh.Select[*authn.Authn], ...huh.Option[*authn.Authn]) (*authn.Authn, error) {
			t.Fatal("unexpected picker invocation")
			return nil, nil
		})
		require.ErrorContains(t, err, "unable to fetch login options")
	})

	t.Run("ssh exchange failure is reported", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			if req.URL.Path == "/authn/s/" {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`{"token":"stub-options-token"}`))), Header: http.Header{}}
			}
			require.Equal(t, "/authn/ssh", req.URL.Path)
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewReader(nil)), Header: http.Header{}}
		})
		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil
		})

		err = Signup{SSHKeyPath: "/path/to/key"}.run(context.Background(), &cmdopts.Global{Context: context.Background()}, oauthc, chttp, signer, func(*huh.Select[*authn.Authn], ...huh.Option[*authn.Authn]) (*authn.Authn, error) {
			t.Fatal("unexpected picker invocation")
			return nil, nil
		})
		require.Error(t, err)
	})

	t.Run("single profile without an account id prompts with the register option first", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": ""}).SignedString([]byte("secret"))
		require.NoError(t, err)
		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: signed})
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			if req.URL.Path == "/authn/s/" {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(options)), Header: http.Header{}}
			}
			require.Equal(t, "/authn/ssh", req.URL.Path)
			encoded, err := json.Marshal(authn.Authed{Profiles: []*authn.Authn{{Token: "token-1", Profile: &authn.Profile{Id: "profile-1", Display: "dave", AccountId: "acct-001"}, Account: &authn.Account{Id: "acct-001", Display: "acme"}}}})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		var requests []string
		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			requests = append(requests, req.URL.Path+" "+req.Header.Get("Authorization"))
			encoded, err := json.Marshal(authn.Current{
				Profile: &authn.Profile{Id: "profile-1", Display: "dave"},
				Account: &authn.Account{Id: "acct-001", Display: "acme"},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		var offered []string
		err = Signup{SSHKeyPath: "/path/to/key"}.run(context.Background(), &cmdopts.Global{Context: context.Background()}, oauthc, chttp, signer, func(prompt *huh.Select[*authn.Authn], options ...huh.Option[*authn.Authn]) (*authn.Authn, error) {
			for _, o := range options {
				offered = append(offered, o.Key)
			}
			return options[1].Value, nil
		})
		require.NoError(t, err)
		require.Equal(t, []string{"register a new account", "acct-001 - acme - dave"}, offered)
		require.Equal(t, []string{"/authn/current BEARER token-1"}, requests)
	})

	t.Run("multiple profiles prompt and log into the chosen profile", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": ""}).SignedString([]byte("secret"))
		require.NoError(t, err)
		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: signed})
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			if req.URL.Path == "/authn/s/" {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(options)), Header: http.Header{}}
			}
			require.Equal(t, "/authn/ssh", req.URL.Path)
			encoded, err := json.Marshal(authn.Authed{Profiles: []*authn.Authn{{Token: "token-1", Profile: &authn.Profile{Id: "profile-1", Display: "dave", AccountId: "acct-001"}, Account: &authn.Account{Id: "acct-001", Display: "acme"}}, {Token: "token-2", Profile: &authn.Profile{Id: "profile-2", Display: "erin", AccountId: "acct-002"}, Account: &authn.Account{Id: "acct-002", Display: "globex"}}}})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		var requests []string
		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			requests = append(requests, req.URL.Path+" "+req.Header.Get("Authorization"))
			encoded, err := json.Marshal(authn.Current{
				Profile: &authn.Profile{Id: "profile-1", Display: "dave"},
				Account: &authn.Account{Id: "acct-001", Display: "acme"},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		var offered []string
		err = Signup{SSHKeyPath: "/path/to/key"}.run(context.Background(), &cmdopts.Global{Context: context.Background()}, oauthc, chttp, signer, func(prompt *huh.Select[*authn.Authn], options ...huh.Option[*authn.Authn]) (*authn.Authn, error) {
			for _, o := range options {
				offered = append(offered, o.Key)
			}
			return options[2].Value, nil
		})
		require.NoError(t, err)
		require.Equal(t, []string{"register a new account", "acct-001 - acme - dave", "acct-002 - globex - erin"}, offered)
		require.Equal(t, []string{"/authn/current BEARER token-2"}, requests)
	})

	t.Run("multiple profiles with an account id still prompt", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": "", "account_id": "acct-002"}).SignedString([]byte("secret"))
		require.NoError(t, err)
		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: signed})
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			if req.URL.Path == "/authn/s/" {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(options)), Header: http.Header{}}
			}
			require.Equal(t, "/authn/ssh", req.URL.Path)
			encoded, err := json.Marshal(authn.Authed{Profiles: []*authn.Authn{{Token: "token-1", Profile: &authn.Profile{Id: "profile-1", Display: "dave", AccountId: "acct-001"}, Account: &authn.Account{Id: "acct-001", Display: "acme"}}, {Token: "token-2", Profile: &authn.Profile{Id: "profile-2", Display: "erin", AccountId: "acct-002"}, Account: &authn.Account{Id: "acct-002", Display: "globex"}}}})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		var requests []string
		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			requests = append(requests, req.URL.Path+" "+req.Header.Get("Authorization"))
			encoded, err := json.Marshal(authn.Current{
				Profile: &authn.Profile{Id: "profile-1", Display: "dave"},
				Account: &authn.Account{Id: "acct-001", Display: "acme"},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		var offered []string
		err = Signup{SSHKeyPath: "/path/to/key"}.run(context.Background(), &cmdopts.Global{Context: context.Background()}, oauthc, chttp, signer, func(prompt *huh.Select[*authn.Authn], options ...huh.Option[*authn.Authn]) (*authn.Authn, error) {
			for _, o := range options {
				offered = append(offered, o.Key)
			}
			return options[1].Value, nil
		})
		require.NoError(t, err)
		require.Equal(t, []string{"register a new account", "acct-001 - acme - dave", "acct-002 - globex - erin"}, offered)
		require.Equal(t, []string{"/authn/current BEARER token-1"}, requests)
	})

	t.Run("choosing to register a new account signs up", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": ""}).SignedString([]byte("secret"))
		require.NoError(t, err)
		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: signed})
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			if req.URL.Path == "/authn/s/" {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(options)), Header: http.Header{}}
			}
			require.Equal(t, "/authn/ssh", req.URL.Path)
			encoded, err := json.Marshal(authn.Authed{Profiles: []*authn.Authn{{Token: "token-1", Profile: &authn.Profile{Id: "profile-1", Display: "dave", AccountId: "acct-001"}, Account: &authn.Account{Id: "acct-001", Display: "acme"}}, {Token: "token-2", Profile: &authn.Profile{Id: "profile-2", Display: "erin", AccountId: "acct-002"}, Account: &authn.Account{Id: "acct-002", Display: "globex"}}}})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		var requests []string
		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			requests = append(requests, req.URL.Path+" "+req.Header.Get("Authorization"))
			encoded, err := json.Marshal(authn.Current{
				Profile: &authn.Profile{Id: "profile-1", Display: "dave"},
				Account: &authn.Account{Id: "acct-001", Display: "acme"},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		var offered []string
		err = Signup{SSHKeyPath: "/path/to/key"}.run(context.Background(), &cmdopts.Global{Context: context.Background()}, oauthc, chttp, signer, func(prompt *huh.Select[*authn.Authn], options ...huh.Option[*authn.Authn]) (*authn.Authn, error) {
			for _, o := range options {
				offered = append(offered, o.Key)
			}
			return options[0].Value, nil
		})
		require.NoError(t, err)
		require.Len(t, requests, 1)
		require.True(t, strings.HasPrefix(requests[0], "/authn/signup BEARER "))
	})

	t.Run("picker failure is propagated", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": ""}).SignedString([]byte("secret"))
		require.NoError(t, err)
		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: signed})
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			if req.URL.Path == "/authn/s/" {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(options)), Header: http.Header{}}
			}
			require.Equal(t, "/authn/ssh", req.URL.Path)
			encoded, err := json.Marshal(authn.Authed{Profiles: []*authn.Authn{{Token: "token-1", Profile: &authn.Profile{Id: "profile-1", Display: "dave", AccountId: "acct-001"}, Account: &authn.Account{Id: "acct-001", Display: "acme"}}, {Token: "token-2", Profile: &authn.Profile{Id: "profile-2", Display: "erin", AccountId: "acct-002"}, Account: &authn.Account{Id: "acct-002", Display: "globex"}}}})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		var requests []string
		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			requests = append(requests, req.URL.Path+" "+req.Header.Get("Authorization"))
			encoded, err := json.Marshal(authn.Current{
				Profile: &authn.Profile{Id: "profile-1", Display: "dave"},
				Account: &authn.Account{Id: "acct-001", Display: "acme"},
			})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})

		var offered []string
		err = Signup{SSHKeyPath: "/path/to/key"}.run(context.Background(), &cmdopts.Global{Context: context.Background()}, oauthc, chttp, signer, func(prompt *huh.Select[*authn.Authn], options ...huh.Option[*authn.Authn]) (*authn.Authn, error) {
			for _, o := range options {
				offered = append(offered, o.Key)
			}
			return nil, errors.New("picker failed")
		})
		require.ErrorContains(t, err, "picker failed")
		require.Empty(t, requests)
	})

	t.Run("undecodable login options are reported", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			if req.URL.Path == "/authn/s/" {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`{"token":"stub-options-token"}`))), Header: http.Header{}}
			}
			require.Equal(t, "/authn/ssh", req.URL.Path)
			encoded, err := json.Marshal(authn.Authed{})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})
		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil
		})

		err = Signup{SSHKeyPath: "/path/to/key"}.run(context.Background(), &cmdopts.Global{Context: context.Background()}, oauthc, chttp, signer, func(*huh.Select[*authn.Authn], ...huh.Option[*authn.Authn]) (*authn.Authn, error) {
			t.Fatal("unexpected picker invocation")
			return nil, nil
		})
		require.ErrorContains(t, err, "unable to decode login options token")
	})

	t.Run("register request failure is reported", func(t *testing.T) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Name()))
		require.NoError(t, err)

		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": "sso-1"}).SignedString([]byte("secret"))
		require.NoError(t, err)
		options, err := json.Marshal(&authn.LoginOptionsResponse{Token: signed})
		require.NoError(t, err)

		oauthc := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			if req.URL.Path == "/authn/s/" {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(options)), Header: http.Header{}}
			}
			require.Equal(t, "/authn/ssh", req.URL.Path)
			encoded, err := json.Marshal(authn.Authed{})
			require.NoError(t, err)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}
		})
		chttp := httptestx.NewTestClient(func(req *http.Request) *http.Response {
			require.Equal(t, "/authn/register", req.URL.Path)
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewReader(nil)), Header: http.Header{}}
		})

		err = Signup{SSHKeyPath: "/path/to/key"}.run(context.Background(), &cmdopts.Global{Context: context.Background()}, oauthc, chttp, signer, func(*huh.Select[*authn.Authn], ...huh.Option[*authn.Authn]) (*authn.Authn, error) {
			t.Fatal("unexpected picker invocation")
			return nil, nil
		})
		require.Error(t, err)
	})
}
