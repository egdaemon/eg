// Package egtarball basic functionality for creating tar balls.
// it proves the following functionality:
// - build deterministic paths to a directory for adding contents.
// - build dterministric archive names from patterns using information provided by the eg environment.
// - common patterns
// Assumptions:
// - tar/gh cli commands are available.
// - the archive patterns used are unique within the repository the workload is associated with.
// Compability guarentee: as long as you only use the functions provided by this package for accessing and generating
// the tarballs we'll ensure no breaking changes.
package egtarball

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egmd5x"
	"github.com/egdaemon/eg/runtime/x/wasi/egsha256x"
)

func root(paths ...string) string {
	return egenv.WorkspaceDirectory(filepath.Join(paths...))
}

// Path generate a unique directory for the contents that will be inside the archive can be
// placed.
func Path(pattern string) string {
	// we dont want a deep tree in the tarball directory and we want them namespaced.
	// create a uuid from the git repository and the paths provided.
	// this will scope the paths to within a single repository in the cache.
	// longer term we'll move this into a 'run scratch pad directory'
	return root(fmt.Sprintf(".eg.tarball.%s", egmd5x.String(filepath.Join(eggit.EnvCanonicalURI(), pattern))))
}

// replaces the substitution values within the pattern, resulting in the final resulting archive file's name.
func Name(pattern string) string {
	c := eggit.EnvCommit()
	return fmt.Sprintf("%s.tar.xz", c.StringReplace(pattern))
}

// simple template for naming a tarball from git commit information. see eggit.commit.StringReplace for details.
func GitPattern(prefix string) string {
	return fmt.Sprintf("%s.%%git.commit.year%%.%%git.commit.month%%.%%git.commit.day%%%%git.hash.short%%", prefix)
}

// Return the path to the archive for the given pattern after Pack has been called.
func Archive(pattern string) string {
	return root(Name(pattern))
}

// Reads the cached sha256 from disk, if it cant locate it, recalculates and stores it.
// will panic if there is an issue calculating the sha256/persisting
func SHA256(pattern string) string {
	path := Archive(pattern)
	sha := fmt.Sprintf("%s.%s", path, "sha256")
	if digest, err := os.ReadFile(sha); err == nil {
		return string(digest)
	}

	digest := egsha256x.DigestFile(path)
	if digest == nil {
		panic(fmt.Errorf("unable to compute the sha256 for %s", path))
	}
	digesthex := egsha256x.FormatHex(digest)
	if strings.TrimSpace(digesthex) == "" {
		panic(fmt.Errorf("unable to format the sha256 for %s", path))
	}

	if err := os.WriteFile(sha, []byte(digesthex), 0644); err != nil {
		panic(err)
	}

	return digesthex
}

func SHA256Op(pattern string) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		if strings.TrimSpace(SHA256(pattern)) == "" {
			return fmt.Errorf("unable to compute sha256: %s", Archive(pattern))
		}
		return nil
	}
}

// create a tarball from the contents of the archive's folder.
func Pack(pattern string) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		dir := Path(pattern)
		name := Name(pattern)
		archive := root(name)

		return shell.Run(
			ctx,
			shell.Newf("tar -C %s -Jcvf %s .", dir, archive),
		)
	}
}
