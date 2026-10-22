// Package gcpx lets a workload exchange a short-lived, egdaemon-issued Google
// ID token for access to a customer's own GCP project via Workload Identity
// Federation: the customer's WIF pool trusts egdaemon's identity service
// account (see egd-workloads), and this package fetches/refreshes that
// identity token and wires it up as standard Google Application Default
// Credentials (ADC) via an "external_account" credential config, the same
// way gitx handles short-lived VCS credentials.
package gcpx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/timex"
)

const (
	// Directory is the directory within the workload's runtime directory holding its gcp credentials.
	Directory = "gcloud"

	responseFilename = "gcpaccess.token"
	idTokenFilename  = "gcpaccess.idtoken"
	adcFilename      = "gcpaccess.adc.json"
)

// GCPDownloadToken builds the claims for the bearer token presented to
// /r/gcpaccess/ -- aid identifies the egdaemon account, label identifies
// which of that account's connected GCP projects (repository.GCPIdentityConnection)
// to mint credentials for.
func GCPDownloadToken(aid string, label string, options ...jwtx.Option) jwt.RegisteredClaims {
	return jwtx.NewJWTClaims(
		label,
		jwtx.ClaimsOptionExpiration(24*time.Hour),
		jwtx.ClaimsOptionIssuer(aid),
		jwtx.ClaimsOptionComposed(options...),
	)
}

// externalAccountConfig is Google's "file-sourced credentials" ADC format for
// external_account, see
// https://google.aip.dev/auth/4117#determining-the-subject-token-in-a-file
type externalAccountConfig struct {
	Type                           string                `json:"type"`
	Audience                       string                `json:"audience"`
	SubjectTokenType               string                `json:"subject_token_type"`
	TokenURL                       string                `json:"token_url"`
	CredentialSource               externalCredentialSrc `json:"credential_source"`
	ServiceAccountImpersonationURL string                `json:"service_account_impersonation_url,omitempty"`
}

type externalCredentialSrc struct {
	File string `json:"file"`
}

// AutomaticCredentialRefresh fetches gcp access credentials immediately and then
// periodically in the background for as long as ctx is alive.
func AutomaticCredentialRefresh(ctx context.Context, c *http.Client, dst string, token string) error {
	if stringsx.Blank(token) {
		debugx.Println("access token blank skipping")
		return nil
	}

	debugx.Println("periodic gcp credentials refresh enabled")
	if err := credentialRefresh(ctx, c, dst, token); err != nil {
		return errorsx.Wrap(err, "failed to initially fetch access token")
	}

	go timex.Every(10*time.Minute, func() {
		errorsx.Log(errorsx.Wrap(credentialRefresh(ctx, c, dst, token), "unable to refresh credentials"))
	})

	return nil
}

// RefreshCredentials performs a single, immediate exchange of the given long-lived
// access token for a short-lived gcp identity token, written to dst (see LoadCredentials).
func RefreshCredentials(ctx context.Context, c *http.Client, dst string, token string) error {
	if stringsx.Blank(token) {
		debugx.Println("access token blank skipping")
		return nil
	}

	return credentialRefresh(ctx, c, dst, token)
}

func credentialRefresh(ctx context.Context, c *http.Client, dst, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/r/gcpaccess/", eg.EnvContainerAPIHostDefault()), nil)
	if err != nil {
		return errorsx.Wrap(err, "unable to create http request")
	}
	req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", token))

	resp, err := httpx.AsError(c.Do(req))
	if err != nil {
		return errorsx.Wrap(err, "http request failed")
	}
	defer resp.Body.Close()
	encoded, err := io.ReadAll(resp.Body)
	if err != nil {
		return errorsx.Wrap(err, "unable to read response")
	}

	if err = os.WriteFile(filepath.Join(dst, responseFilename), encoded, 0666); err != nil {
		return errorsx.Wrap(err, "unable to write credentials")
	}

	return nil
}

// LoadCredentials reads the most recently fetched gcp access credentials and writes
// them out as a standard Google ADC external_account credential config, returning
// its path. Set GOOGLE_APPLICATION_CREDENTIALS to that path so any Google client
// library or gcloud invocation transparently federates into the customer's project.
func LoadCredentials(ctx context.Context, dir string) (string, error) {
	var creds compute.GCPAccessCredentials

	encoded, err := os.ReadFile(filepath.Join(dir, responseFilename))
	if err != nil {
		return "", err
	}

	if err = json.Unmarshal(encoded, &creds); err != nil || stringsx.Blank(creds.IdToken) {
		return "", nil
	}

	idtokenpath := filepath.Join(dir, idTokenFilename)
	if err = os.WriteFile(idtokenpath, []byte(creds.IdToken), 0600); err != nil {
		return "", errorsx.Wrap(err, "unable to write identity token")
	}

	cfg := externalAccountConfig{
		Type:             "external_account",
		Audience:         creds.Audience,
		SubjectTokenType: "urn:ietf:params:oauth:token-type:jwt",
		TokenURL:         "https://sts.googleapis.com/v1/token",
		CredentialSource: externalCredentialSrc{File: idtokenpath},
	}

	if stringsx.Present(creds.ImpersonateServiceAccount) {
		cfg.ServiceAccountImpersonationURL = fmt.Sprintf(
			"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken",
			creds.ImpersonateServiceAccount,
		)
	}

	encodedcfg, err := json.Marshal(cfg)
	if err != nil {
		return "", errorsx.Wrap(err, "unable to encode adc config")
	}

	adcpath := filepath.Join(dir, adcFilename)
	if err = os.WriteFile(adcpath, encodedcfg, 0600); err != nil {
		return "", errorsx.Wrap(err, "unable to write adc config")
	}

	return adcpath, nil
}

// ConfigureGCP points gcloud at the application default credentials installed at adc. gcloud commands (storage,
// compute, ...) only use credentials of their own account and ignore the ADC unless auth/credential_file_override
// is set, which the google client libraries do not need. dir is the gcloud configuration directory of the user.
func ConfigureGCP(dir string, adc string) error {
	configs := filepath.Join(dir, "configurations")
	if err := fsx.MkDirs(0770, configs); err != nil {
		return errorsx.Wrapf(err, "unable to create %s", configs)
	}

	return errorsx.Wrap(os.WriteFile(filepath.Join(configs, "config_default"), []byte("[auth]\ncredential_file_override = "+adc+"\n"), 0600), "unable to write gcloud configuration")
}

// fetch exchanges the workload's access token for gcp credentials, written to dst.
type fetch func(ctx context.Context, c *http.Client, dst string, token string) error

// wanted reports whether the workload enabled gcp workload identity in its environment file, see
// eg.EnvGCPWorkloadIdentityEnabled.
func wanted(environpath string) bool {
	environ, err := envx.Build().FromPath(environpath).Environ()
	if err != nil {
		return false
	}

	for _, kv := range environ {
		if k, v, _ := strings.Cut(kv, "="); k == eg.EnvGCPWorkloadIdentityEnabled {
			enabled, _ := strconv.ParseBool(v)
			return enabled
		}
	}

	return false
}

// ProvisionCredentials provisions the gcp credentials of the workload's account when the workload enabled them,
// into the Directory of the workload's runtime directory, which the container installs as the application default
// credentials of the unprivileged user. the credentials are refreshed until ctx is done or the runtime directory
// is removed.
//
// the path of the application default credentials as seen by the host is returned, it is empty when the workload
// did not enable them or when the account has no gcp identity connection.
func ProvisionCredentials(ctx context.Context, c *http.Client, runtimedir string, token string) (string, error) {
	return provision(ctx, RefreshCredentials, c, runtimedir, token)
}

func provision(ctx context.Context, get fetch, c *http.Client, runtimedir string, token string) (adc string, err error) {
	dir := filepath.Join(runtimedir, Directory)

	if !wanted(filepath.Join(runtimedir, eg.EnvironFile)) {
		log.Println("debug: gcp workload identity not requested by the workload,", eg.EnvGCPWorkloadIdentityEnabled, "is not true")
		return "", nil
	}

	if err = fsx.MkDirs(0770, dir); err != nil {
		return "", errorsx.Wrap(err, "unable to create gcp credentials directory")
	}

	refresh := func() (string, error) {
		if err := get(ctx, c, dir, token); err != nil {
			return "", err
		}

		return LoadCredentials(ctx, dir)
	}

	if adc, err = refresh(); httpx.IsStatusError(err, http.StatusUnauthorized, http.StatusNotFound) != nil {
		// no gcp identity connection, nothing to install.
		log.Println("debug: no gcp identity connection for the account", err)
		return "", errorsx.Wrap(os.RemoveAll(dir), "unable to clear gcp credentials directory")
	} else if err != nil {
		return "", errorsx.Wrap(err, "unable to provision gcp credentials")
	}

	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			if _, err := os.Stat(dir); err != nil {
				return
			}

			_, err := refresh()
			errorsx.Log(errorsx.Wrap(err, "unable to refresh gcp credentials"))
		}
	}()

	return adc, nil
}
