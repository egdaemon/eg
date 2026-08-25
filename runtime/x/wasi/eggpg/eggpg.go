package eggpg

import (
	"context"
	"errors"
	"fmt"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

const (
	EnvHome  = "GNUPGHOME"
	EnvName  = "EG_GPG_KEYRING_NAME"
	EnvEmail = "EG_GPG_KEYRING_EMAIL"
	EnvSeed  = "EG_GPG_KEYRING_SEED"
)

type Option func(*option)
type options []Option

type option struct {
	home  string
	name  string
	email string
	seed  string
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

func runtime(options ...Option) (_ shell.Command, err error) {
	var opts option
	for _, opt := range options {
		opt(&opts)
	}

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
	)

	if err != nil {
		return shell.Command{}, err
	}

	return shell.Env().
		Environ(EnvHome, opts.home).
		Environ(EnvEmail, opts.email).
		Environ(EnvName, opts.name).
		Environ(EnvSeed, opts.seed), nil
}

func Debug(options ...Option) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		runtime, err := runtime(options...)
		if err != nil {
			return err
		}
		runtime = runtime.Lenient(true)
		return shell.Op(
			runtime.New("env | grep -i 'GNUPG'"),
			runtime.New("env | grep -i 'EG_GPG_'"),
			runtime.New("ls -lha ${GNUPGHOME}"),
			runtime.New("gpg --version"),
			runtime.New("gpg-agent --version"),
			runtime.New("gpg --list-keys"),
		)(ctx, o)
	}
}

// Generate a usable gpg keyring from a seed.
func Seed(options ...Option) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		runtime, err := runtime(options...)
		if err != nil {
			return err
		}
		return shell.Op(
			runtime.New("eg gpg keyring --directory=\"${GNUPGHOME}\" --name=\"${EG_GPG_KEYRING_NAME}\" --email=\"${EG_GPG_KEYRING_EMAIL}\" --seed=\"${EG_GPG_KEYRING_SEED}\""),
			runtime.New("gpg --import ${GNUPGHOME}/private.asc"),
		)(ctx, o)
	}
}
