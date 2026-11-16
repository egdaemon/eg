package cmdgpg

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/gpgx"
)

type Cmd struct {
	Keyring CmdKeyring `cmd:"" name:"keyring" help:"generate a deterministic gpg keyring from a seed"`
}

// CmdKeyring the env tags are the variables eggpg reads (eg.EnvGPGKeyring*), so the same environment that
// seeds a keyring inside a workload generates the identical key here. flags take precedence over the environment.
type CmdKeyring struct {
	Seed      string `name:"seed" help:"seed for generating a deterministic keyring, useful for ci/cd" default:"${vars_entropy_seed}" env:"${env_eg_gpg_keyring_seed}"`
	Directory string `name:"directory" short:"d" help:"directory to write private.asc and public.asc into" default:"${vars_gpg_directory}" env:"${env_eg_gpg_keyring_home}"`
	Name      string `name:"name" help:"identity name to embed in the key" default:"${vars_user_name}" env:"${env_eg_gpg_keyring_name}"`
	Email     string `name:"email" help:"identity email to embed in the key" default:"" required:"" env:"${env_eg_gpg_keyring_email}"`
}

// Run generates a deterministic GPG keyring from the given seed and writes
// private.asc and public.asc into the target directory.
//
// The generated files are armored PGP keys. GPG does not load them
// automatically — you must import the private key before gpg commands can use
// it:
//
//	GNUPGHOME=<directory> gpg --import <directory>/private.asc
//
// The key is stable: running the command again with the same seed produces the
// same key ID, so a second call with an existing directory is a no-op (the
// existing files are loaded from disk unchanged).
func (t CmdKeyring) Run(gctx *cmdopts.Global) error {
	entity, err := gpgx.Keyring(t.Directory, t.Seed, gpgx.OptionKeyGenIdentity(t.Name, "", t.Email))
	if err != nil {
		return fmt.Errorf("gpg keyring generation failed: %w", err)
	}

	debug(t.Directory)

	fmt.Printf("keyring written to %s (key id: %016X, fingerprint: %X)\n", t.Directory, entity.PrimaryKey.KeyId, entity.PrimaryKey.Fingerprint)
	fmt.Printf("to use with gpg: GNUPGHOME=%s gpg --import %s/private.asc\n", t.Directory, t.Directory)

	return nil
}

// debug prints the same diagnostics as eggpg.Debug does for a workload, to stderr so stdout stays the result:
// the gnupg environment, the keyring files and the gpg installation, with GNUPGHOME set to the keyring directory. it is lenient, a failing diagnostic is
// reported and never fails the command. the seed is redacted.
func debug(directory string) {
	// gpg is pointed at the keyring directory, not whatever GNUPGHOME the caller has.
	environment := []string{"GNUPGHOME=" + directory}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GNUPGHOME=") {
			environment = append(environment, kv)
		}
	}

	section := func(title string) {
		fmt.Fprintf(os.Stderr, "\n# %s\n", title)
	}

	environ := func(pattern string) {
		for _, kv := range environment {
			if !strings.Contains(strings.ToLower(kv), strings.ToLower(pattern)) {
				continue
			}

			if strings.HasPrefix(kv, eg.EnvGPGKeyringSeed+"=") {
				kv = eg.EnvGPGKeyringSeed + "=<redacted>"
			}

			fmt.Fprintln(os.Stderr, kv)
		}
	}

	run := func(name string, args ...string) {
		section(strings.Join(append([]string{name}, args...), " "))
		cmd := exec.Command(name, args...)
		cmd.Env = environment
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "debug: %s failed: %v\n", name, err)
		}
	}

	section("env | grep -i GNUPG")
	environ("GNUPG")
	section("env | grep -i EG_GPG_")
	environ("EG_GPG_")

	run("ls", "-lha", directory)

	run("gpg", "--version")
	run("gpg-agent", "--version")
	run("gpg", "--list-keys")

	section("sha256sum private.asc public.asc")
	for _, name := range []string{"private.asc", "public.asc"} {
		path := filepath.Join(directory, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "debug: unable to read %s: %v\n", path, err)
			continue
		}

		fmt.Fprintf(os.Stderr, "%x  %s\n", sha256.Sum256(raw), path)
	}
}
