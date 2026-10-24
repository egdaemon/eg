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
	"os/user"
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
	"github.com/egdaemon/eg/internal/generations"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/timex"
)

const (
	// refreshInterval is how often the runner fetches new credentials for the workload. the identity tokens last an hour.
	refreshInterval = 20 * time.Minute
	// syncInterval is how often the workload's container publishes the fetched credentials for the user, the credentials
	// are at most refreshInterval + syncInterval old.
	syncInterval = 5 * time.Minute
)

const (
	// Directory is the directory within the workload's runtime directory holding its gcp credentials.
	Directory = "gcloud"

	responseFilename = "gcpaccess.token"
	idTokenFilename  = "gcpaccess.idtoken"
	adcFilename      = "gcpaccess.adc.json"

	adcInstalledFilename = "application_default_credentials.json"

	generationsDirectory = "eg" // within the gcloud configuration directory of the user, holds the generations of the credentials.
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

	if err = generations.WriteFile(filepath.Join(dst, responseFilename), encoded, 0666); err != nil {
		return errorsx.Wrap(err, "unable to write credentials")
	}

	return nil
}

// LoadCredentials reads the most recently fetched gcp access credentials and writes
// them out as a standard Google ADC external_account credential config, returning
// its path. Set GOOGLE_APPLICATION_CREDENTIALS to that path so any Google client
// library or gcloud invocation transparently federates into the customer's project.
func LoadCredentials(ctx context.Context, dir string) (string, error) {
	return load(dir, dir, dir)
}

// load reads the fetched credentials from the from directory and writes the identity token and the adc into the to
// directory. the adc reads the identity token from the published directory, which is where the files end up when
// to is not their final location. the path of the adc is returned, it is empty when there are no credentials.
func load(from, to, published string) (string, error) {
	var creds compute.GCPAccessCredentials

	encoded, err := os.ReadFile(filepath.Join(from, responseFilename))
	if err != nil {
		return "", err
	}

	if err = json.Unmarshal(encoded, &creds); err != nil || stringsx.Blank(creds.IdToken) {
		return "", nil
	}

	if err = generations.WriteFile(filepath.Join(to, idTokenFilename), []byte(creds.IdToken), 0600); err != nil {
		return "", errorsx.Wrap(err, "unable to write identity token")
	}

	cfg := externalAccountConfig{
		Type:             "external_account",
		Audience:         creds.Audience,
		SubjectTokenType: "urn:ietf:params:oauth:token-type:jwt",
		TokenURL:         "https://sts.googleapis.com/v1/token",
		CredentialSource: externalCredentialSrc{File: filepath.Join(published, idTokenFilename)},
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

	adcpath := filepath.Join(to, adcFilename)
	if err = generations.WriteFile(adcpath, encodedcfg, 0600); err != nil {
		return "", errorsx.Wrap(err, "unable to write adc config")
	}

	return adcpath, nil
}

// InstallCredentials installs the credentials provided to the workload as the application default credentials of the
// unprivileged user, at ~/.config/gcloud/application_default_credentials.json where the google client libraries find
// them, and returns that path. gcloud commands only use them when pointed at the file, see eg.EnvCloudSDKCredentialFileOverride.
//
// the credentials are either issued by the runner (workload identity), found in the src directory, or provided by the
// agent as eg.EnvUnsafeGcloudADCB64 (eg compute local|serve --gcp-auto). nothing is installed, and the returned path
// is empty, when the workload has neither. the credentials issued by the runner are refreshed until ctx is done.
func InstallCredentials(ctx context.Context, src string) (string, error) {
	egdu, err := user.Lookup(eg.DefaultUsername)
	if err != nil {
		return "", errorsx.Wrap(err, "failed to lookup user egd")
	}

	uid, gid := errorsx.Zero(strconv.Atoi(egdu.Uid)), errorsx.Zero(strconv.Atoi(egdu.Gid))

	return install(ctx, filepath.Join(egdu.HomeDir, ".config", "gcloud"), src, envx.Base64URL(nil, eg.EnvUnsafeGcloudADCB64), syncInterval, func(path string) error {
		return os.Chown(path, uid, gid)
	})
}

// install publishes the credentials into the generations directory within dst, see publish, and links the application
// default credentials to the current generation. nothing within dst is replaced or hidden, the configuration of the
// user and the defaults of the image stay as they are.
func install(ctx context.Context, dst string, src string, provided []byte, every time.Duration, chown func(path string) error) (string, error) {
	var (
		installed = filepath.Join(dst, adcInstalledFilename)
		base      = filepath.Join(dst, generationsDirectory)
		current   = filepath.Join(base, generations.Current)
	)

	if fsx.DirExists(src) != nil {
		if len(provided) == 0 {
			return "", nil
		}

		if _, err := generations.Publish(ctx, base, chown, func(dir string) (bool, error) {
			return true, generations.WriteFile(filepath.Join(dir, adcInstalledFilename), provided, 0600)
		}); err != nil {
			return "", errorsx.Wrap(err, "unable to publish application default credentials")
		}

		return installed, errorsx.Wrap(generations.Link(installed, filepath.Join(generationsDirectory, generations.Current, adcInstalledFilename)), "unable to link application default credentials")
	}

	refresh := func() (bool, error) {
		return generations.Publish(ctx, base, chown, func(dir string) (bool, error) {
			adc, err := load(src, dir, current)
			if err != nil || adc == "" {
				return false, err
			}

			return true, os.Rename(adc, filepath.Join(dir, adcInstalledFilename))
		})
	}

	if ok, err := refresh(); err != nil {
		return "", errorsx.Wrap(err, "unable to publish gcp credentials")
	} else if !ok {
		return "", nil
	}

	if err := generations.Link(installed, filepath.Join(generationsDirectory, generations.Current, adcInstalledFilename)); err != nil {
		return "", errorsx.Wrap(err, "unable to link application default credentials")
	}

	go func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			_, err := refresh()
			errorsx.Log(errorsx.Wrap(err, "unable to refresh gcp credentials"))
		}
	}()

	return installed, nil
}

// provided reports whether the agent provided the credentials of the workload, see eg.EnvUnsafeGcloudADCB64, by
// reading the workload's environment file.
func provided(environpath string) bool {
	environ, err := envx.Build().FromPath(environpath).Environ()
	if err != nil {
		return false
	}

	for _, kv := range environ {
		if k, v, _ := strings.Cut(kv, "="); k == eg.EnvUnsafeGcloudADCB64 {
			return stringsx.Present(v)
		}
	}

	return false
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

	if environpath := filepath.Join(runtimedir, eg.EnvironFile); provided(environpath) {
		log.Println("debug: gcp credentials provided by the agent, not requesting workload identity")
		return "", nil
	} else if !wanted(environpath) {
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
		ticker := time.NewTicker(refreshInterval)
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
