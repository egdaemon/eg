package eggpg

import (
	"context"
	"errors"
	"fmt"

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

type Option func(*option)
type options []Option

type option struct {
	keyringhome string
	home        string
	name        string
	email       string
	seed        string
	debug       bool
}

// Generates default options from the environment
func Options() options {
	return options(nil).
		Home(egenv.String("/home/egd/.gnupg", EnvHome)).
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

func autokeyringhome(o *option) {
	root := md5x.FormatString(md5x.Digest("gnupg", egenv.RunID(), o.seed))
	o.keyringhome = langx.FirstNonZero(
		o.keyringhome,
		egenv.WorkspaceDirectory(fmt.Sprintf("gnupg.%s", root)),
	)
}

func parseOptions(options ...Option) (opts option, err error) {
	opts = langx.Clone(opts, options...)
	opts = langx.Clone(opts, autokeyringhome)

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

func (opts option) runtime() shell.Command {
	return shell.Env().
		MaybeDebug(opts.debug).
		Environ(envKeyringHome, opts.keyringhome).
		Environ(EnvHome, opts.home).
		Environ(EnvEmail, opts.email).
		Environ(EnvName, opts.name).
		Environ(EnvSeed, opts.seed)
}

func runtime(options ...Option) (_ shell.Command, err error) {
	opts, err := parseOptions(options...)
	if err != nil {
		return shell.Command{}, err
	}

	return opts.runtime(), nil
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
func Seed(options ...Option) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		opts, err := parseOptions(options...)
		if err != nil {
			return err
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
