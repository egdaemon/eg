// Package awsx lets a workload assume a customer's iam role through web identity federation: egdaemon issues a
// short-lived OpenID Connect identity token for the workload's account, which the customer's aws account trusts, and
// this package fetches, refreshes and installs it where the aws sdks and cli find it. there is nothing to configure:
// the sdks exchange the token for the role themselves when AWS_ROLE_ARN and AWS_WEB_IDENTITY_TOKEN_FILE are set,
// see Settings.Environ. it mirrors gcpx.
package awsx

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

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/generations"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/stringsx"
)

const (
	// refreshInterval is how often the runner fetches a new identity token for the workload. the tokens last an hour.
	refreshInterval = 20 * time.Minute
	// syncInterval is how often the workload's container publishes the fetched token for the user, the token is at
	// most refreshInterval + syncInterval old.
	syncInterval = 5 * time.Minute
)

const (
	// Directory is the directory within the workload's runtime directory holding its aws credentials.
	Directory = "aws"

	responseFilename    = "awsaccess.response"
	tokenFilename       = "token"
	credentialsFilename = "credentials"

	generationsDirectory = "eg" // within the aws configuration directory of the user, holds the generations of the credentials.
)

// Settings are the aws configuration of a workload. the aws sdks and cli read it from the environment.
type Settings struct {
	RoleARN         string // the role to assume with the web identity token.
	TokenFile       string // the file holding the web identity token, it is re-read whenever the sdks refresh.
	Region          string
	SessionName     string // shows in cloudtrail.
	CredentialsFile string // shared credentials file, for credentials provided by the agent.
}

// Environ the settings as environment variables, none for settings that are blank.
func (t Settings) Environ() []string {
	var environ []string
	add := func(k, v string) {
		if stringsx.Blank(v) {
			return
		}

		environ = append(environ, fmt.Sprintf("%s=%s", k, v))
	}

	if stringsx.Present(t.RoleARN) && stringsx.Present(t.TokenFile) {
		add(eg.EnvAWSRoleARN, t.RoleARN)
		add(eg.EnvAWSWebIdentityTokenFile, t.TokenFile)
		add(eg.EnvAWSRoleSessionName, t.SessionName)
	}

	add(eg.EnvAWSRegion, t.Region)
	add("AWS_SHARED_CREDENTIALS_FILE", t.CredentialsFile)

	return environ
}

// StaticCredentials are the credentials of a developer, provided to a workload by the agent
// (eg compute local|serve --aws-auto) as eg.EnvUnsafeAWSCredentialsB64.
type StaticCredentials struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token,omitempty"`
}

func (t StaticCredentials) ini() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[default]\naws_access_key_id = %s\naws_secret_access_key = %s\n", t.AccessKeyID, t.SecretAccessKey)
	if t.SessionToken != "" {
		fmt.Fprintf(&b, "aws_session_token = %s\n", t.SessionToken)
	}

	return b.String()
}

// RefreshCredentials performs a single, immediate exchange of the given long-lived access token for a short-lived
// aws identity token, written to dst.
func RefreshCredentials(ctx context.Context, c *http.Client, dst string, token string) error {
	if stringsx.Blank(token) {
		debugx.Println("access token blank skipping")
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/r/awsaccess/", eg.EnvContainerAPIHostDefault()), nil)
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

	return errorsx.Wrap(generations.WriteFile(filepath.Join(dst, responseFilename), encoded, 0600), "unable to write credentials")
}

// load reads the fetched response from the from directory and writes the identity token into the to directory.
// the settings are empty when there are no credentials, the token file is where the token was written.
func load(from, to string) (Settings, error) {
	var creds compute.AWSAccessCredentials

	encoded, err := os.ReadFile(filepath.Join(from, responseFilename))
	if err != nil {
		return Settings{}, err
	}

	if err = json.Unmarshal(encoded, &creds); err != nil || stringsx.Blank(creds.IdToken) || stringsx.Blank(creds.RoleArn) {
		return Settings{}, nil
	}

	tokenfile := filepath.Join(to, tokenFilename)
	if err = generations.WriteFile(tokenfile, []byte(creds.IdToken), 0600); err != nil {
		return Settings{}, errorsx.Wrap(err, "unable to write identity token")
	}

	return Settings{RoleARN: creds.RoleArn, TokenFile: tokenfile, Region: creds.Region}, nil
}

// InstallCredentials installs the credentials provided to the workload for the unprivileged user, under
// ~/.aws/eg where nothing of the user's own configuration is replaced, and returns the settings that make the aws
// sdks and cli use them, see Settings.Environ.
//
// the credentials are either issued by the runner (web identity), found in the src directory, or provided by the
// agent as eg.EnvUnsafeAWSCredentialsB64 (eg compute local|serve --aws-auto). nothing is installed, and the returned
// settings are empty, when the workload has neither. the web identity token is refreshed until ctx is done.
func InstallCredentials(ctx context.Context, src string) (Settings, error) {
	egdu, err := user.Lookup(eg.DefaultUsername)
	if err != nil {
		return Settings{}, errorsx.Wrap(err, "failed to lookup user egd")
	}

	uid, gid := errorsx.Zero(strconv.Atoi(egdu.Uid)), errorsx.Zero(strconv.Atoi(egdu.Gid))
	session := "eg-" + envx.String("workload", eg.EnvComputeRunID)

	return install(ctx, filepath.Join(egdu.HomeDir, ".aws"), src, envx.Base64URL(nil, eg.EnvUnsafeAWSCredentialsB64), session, syncInterval, func(path string) error {
		return os.Chown(path, uid, gid)
	})
}

func install(ctx context.Context, dst string, src string, provided []byte, session string, every time.Duration, chown func(path string) error) (Settings, error) {
	var (
		base    = filepath.Join(dst, generationsDirectory)
		current = filepath.Join(base, generations.Current)
	)

	if fsx.DirExists(src) != nil {
		if len(provided) == 0 {
			return Settings{}, nil
		}

		var creds StaticCredentials
		if err := json.Unmarshal(provided, &creds); err != nil || stringsx.Blank(creds.AccessKeyID) || stringsx.Blank(creds.SecretAccessKey) {
			return Settings{}, errorsx.Compact(errorsx.New("the aws credentials provided by the agent are not complete"), err)
		}

		if _, err := generations.Publish(ctx, base, chown, func(dir string) (bool, error) {
			return true, generations.WriteFile(filepath.Join(dir, credentialsFilename), []byte(creds.ini()), 0600)
		}); err != nil {
			return Settings{}, errorsx.Wrap(err, "unable to publish aws credentials")
		}

		return Settings{CredentialsFile: filepath.Join(current, credentialsFilename)}, nil
	}

	var settings Settings
	refresh := func() (bool, error) {
		return generations.Publish(ctx, base, chown, func(dir string) (bool, error) {
			s, err := load(src, dir)
			if err != nil || s.RoleARN == "" {
				return false, err
			}

			// the token is read through current, which follows every generation.
			settings = Settings{RoleARN: s.RoleARN, TokenFile: filepath.Join(current, tokenFilename), Region: s.Region, SessionName: session}
			return true, nil
		})
	}

	if ok, err := refresh(); err != nil {
		return Settings{}, errorsx.Wrap(err, "unable to publish aws credentials")
	} else if !ok {
		return Settings{}, nil
	}

	initial := settings

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
			errorsx.Log(errorsx.Wrap(err, "unable to refresh aws credentials"))
		}
	}()

	return initial, nil
}

// provided reports whether the agent provided the credentials of the workload, see eg.EnvUnsafeAWSCredentialsB64, by
// reading the workload's environment file.
func provided(environpath string) bool {
	return present(environpath, eg.EnvUnsafeAWSCredentialsB64, stringsx.Present)
}

// wanted reports whether the workload enabled aws workload identity in its environment file, see
// eg.EnvAWSWorkloadIdentityEnabled.
func wanted(environpath string) bool {
	return present(environpath, eg.EnvAWSWorkloadIdentityEnabled, func(v string) bool {
		enabled, _ := strconv.ParseBool(v)
		return enabled
	})
}

func present(environpath, key string, ok func(string) bool) bool {
	environ, err := envx.Build().FromPath(environpath).Environ()
	if err != nil {
		return false
	}

	for _, kv := range environ {
		if k, v, _ := strings.Cut(kv, "="); k == key {
			return ok(v)
		}
	}

	return false
}

// fetch exchanges the workload's access token for aws credentials, written to dst.
type fetch func(ctx context.Context, c *http.Client, dst string, token string) error

// ProvisionCredentials provisions the aws credentials of the workload's account when the workload enabled them,
// into the Directory of the workload's runtime directory, which the container installs for the unprivileged user.
// the credentials are refreshed until ctx is done or the runtime directory is removed.
//
// the settings, as seen by the host, are empty when the workload did not enable them or when the account has no aws
// identity connection.
func ProvisionCredentials(ctx context.Context, c *http.Client, runtimedir string, token string) (Settings, error) {
	return provision(ctx, RefreshCredentials, c, runtimedir, token)
}

func provision(ctx context.Context, get fetch, c *http.Client, runtimedir string, token string) (Settings, error) {
	dir := filepath.Join(runtimedir, Directory)

	if environpath := filepath.Join(runtimedir, eg.EnvironFile); provided(environpath) {
		log.Println("debug: aws credentials provided by the agent, not requesting workload identity")
		return Settings{}, nil
	} else if !wanted(environpath) {
		return Settings{}, nil
	}

	if err := fsx.MkDirs(0770, dir); err != nil {
		return Settings{}, errorsx.Wrap(err, "unable to create aws credentials directory")
	}

	refresh := func() (Settings, error) {
		if err := get(ctx, c, dir, token); err != nil {
			return Settings{}, err
		}

		return load(dir, dir)
	}

	settings, err := refresh()
	if httpx.IsStatusError(err, http.StatusUnauthorized, http.StatusNotFound) != nil {
		// no aws identity connection, nothing to install.
		log.Println("debug: no aws identity connection for the account", err)
		return Settings{}, errorsx.Wrap(os.RemoveAll(dir), "unable to clear aws credentials directory")
	} else if err != nil {
		return Settings{}, errorsx.Wrap(err, "unable to provision aws credentials")
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
			errorsx.Log(errorsx.Wrap(err, "unable to refresh aws credentials"))
		}
	}()

	return settings, nil
}
