// Package eggpg generates and imports a deterministic GPG keyring for a
// pipeline run.
//
// Known issue: libgcrypt < 1.12.1 has a regression in
// gcry_mpi_ec_curve_point (upstream bug T8080: "gcry_mpi_ec_curve_point
// corrupts point") where _gcry_mpi_ec_get_affine mutates the input point's
// MPI coordinates in place instead of copying them first. gpg-agent hits
// this during the self-test it runs on secret key import, which can corrupt
// the point being checked and reject an otherwise-valid ECDH subkey with
// "Bad secret key" / "Point 'G' does not belong to curve 'E'!". Whether a
// given key triggers it depends on that key's specific MPI coordinate
// sizing, so it reproduces deterministically for a given seed but not
// predictably across seeds. Fixed upstream in libgcrypt 1.12.1
// (2026-02-20); the fix is to run gpg/gpg-agent linked against
// libgcrypt >= 1.12.1 — there is no workaround available in this package
// (the keyring generation and import logic here are not the cause).
package eggpg

import (
	"context"
	"errors"
	"fmt"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gpgx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

const (
	EnvHome        = "GNUPGHOME"
	EnvName        = "EG_GPG_KEYRING_NAME"
	EnvEmail       = "EG_GPG_KEYRING_EMAIL"
	EnvSeed        = "EG_GPG_KEYRING_SEED"
	envKeyringHome = "EG_GPG_KEYRING_HOME"
)

// the well known isolated gnupg home used so Seed never touches a real
// user's personal keyring.
const defaultHome = "/home/egd/.gnupg"

type Option func(*option)
type options []Option

type option struct {
	keyringhome    string
	home           string
	name           string
	email          string
	seed           string
	debug          bool
	ignorelocalgnu bool
}

// Generates default options from the environment
func Options() options {
	return options(nil).
		Home(egenv.String(defaultHome, EnvHome)).
		Name(egenv.String("", EnvName)).
		Email(egenv.String("", EnvEmail)).
		Seed(egenv.String("", EnvSeed))
}

func (t options) Home(v string) options {
	return append(t, func(o *option) {
		o.home = v
	})
}

func (t options) Name(v string) options {
	return append(t, func(o *option) {
		o.name = v
	})
}

func (t options) Email(v string) options {
	return append(t, func(o *option) {
		o.email = v
	})
}

func (t options) Seed(v string) options {
	return append(t, func(o *option) {
		o.seed = v
	})
}

func (t options) Debug() options {
	return append(t, func(o *option) {
		o.debug = true
	})
}

// IgnoreLocalGNU makes Seed a no-op whenever the resolved GNUPGHOME isn't the
// known-safe default — i.e. if we can't be sure this isn't someone's real
// local keyring, don't touch it.
func (t options) IgnoreLocalGNU() options {
	return append(t, func(o *option) {
		o.ignorelocalgnu = true
	})
}

func autokeyringhome(o *option) {
	root := md5x.FormatString(md5x.Digest("gnupg", egenv.RunID(), o.seed))
	o.keyringhome = langx.FirstNonZero(
		o.keyringhome,
		egenv.WorkspaceDirectory(fmt.Sprintf("gnupg.%s", root)),
	)
}

func apply(options ...Option) (opts option) {
	opts = langx.Clone(opts, options...)
	opts = langx.Clone(opts, autokeyringhome)
	return opts
}

func parseOptions(options ...Option) (opts option, err error) {
	opts = apply(options...)

	emptycheck := func(v, key string) error {
		if v == "" {
			return fmt.Errorf("%s must not be empty", key)
		}
		return nil
	}

	err = errors.Join(
		emptycheck(opts.home, EnvHome),
		emptycheck(opts.name, EnvName),
		emptycheck(opts.email, EnvEmail),
		emptycheck(opts.seed, EnvSeed),
		emptycheck(opts.keyringhome, envKeyringHome),
	)

	return opts, err
}

func (opts option) env() []string {
	return errorsx.Zero(envx.Build().
		Var(envKeyringHome, opts.keyringhome).
		Var(EnvHome, opts.home).
		Var(EnvEmail, opts.email).
		Var(EnvName, opts.name).
		Var(EnvSeed, opts.seed).Environ())
}

func (opts option) runtime() shell.Command {
	return shell.Env().
		MaybeDebug(opts.debug).
		EnvironFrom(opts.env()...)
}

func runtime(options ...Option) (_ shell.Command, err error) {
	opts, err := parseOptions(options...)
	if err != nil {
		return shell.Command{}, err
	}

	return opts.runtime(), nil
}

func Env(options ...Option) []string {
	opts := errorsx.Must(parseOptions(options...))
	if opts.ignorelocalgnu && opts.home != defaultHome {
		return apply(Options()...).env()
	}

	return opts.env()
}

func Debug(options ...Option) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		runtime, err := runtime(options...)
		if err != nil {
			return err
		}
		runtime = runtime.Lenient(true)
		return shell.Run(
			ctx,
			runtime.New("env | grep -i 'GNUPG'"),
			runtime.New("env | grep -i 'EG_GPG_'"),
			runtime.New("ls -lha ${GNUPGHOME}"),
			runtime.New("gpg --version"),
			runtime.New("gpg-agent --version"),
			runtime.New("gpg --list-keys"),
			runtime.New("sha256sum ${EG_GPG_KEYRING_HOME}/private.asc ${EG_GPG_KEYRING_HOME}/public.asc"),
		)
	}
}

// Generate a usable gpg keyring from a seed.
//
// If the final "gpg --import" fails with "Bad secret key" / "Point 'G' does
// not belong to curve 'E'!", see the package doc — that's the libgcrypt
// <1.12.1 T8080 regression, not a bug in this function.
func Seed(options ...Option) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		opts, err := parseOptions(options...)
		if err != nil {
			return err
		}

		if opts.ignorelocalgnu && opts.home != defaultHome {
			return nil
		}

		if _, err := gpgx.Keyring(opts.keyringhome, opts.seed, gpgx.OptionKeyGenIdentity(opts.name, "", opts.email)); err != nil {
			return err
		}

		runtime := opts.runtime()
		return shell.Run(
			ctx,
			// launch the gpg agent for this environment ensuring its available prior to importing.
			runtime.New("gpgconf --launch gpg-agent"),
			runtime.New("gpg-connect-agent /bye").Attempts(32),
			runtime.New("gpg --import ${EG_GPG_KEYRING_HOME}/private.asc"),
		)
	}
}
