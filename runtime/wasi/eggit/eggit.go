// Package eggit provides functionality around git and assumes that the git command is available.
package eggit

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/patternx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffigit"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

const hashSize = 20

type hash [hashSize]byte

// NewHash return a new Hash from a hexadecimal hash representation
func nhash(s string) hash {
	b, _ := hex.DecodeString(s)

	var h hash
	copy(h[:], b)

	return h
}

func (h hash) IsZero() bool {
	var empty hash
	return h == empty
}

func (h hash) String() string {
	return hex.EncodeToString(h[:])
}

func (h hash) Format(s fmt.State, verb rune) {
	switch verb {
	case 's':
		w := hashSize * 2
		if cw, ok := s.Width(); ok {
			w = cw
		}
		encoded := []rune(hex.EncodeToString(h[:]))
		_ = errorsx.Must(s.Write([]byte(string(encoded[:w]))))
	default:
		_ = errorsx.Must(s.Write([]byte(h.String())))
	}
}

type signature struct {
	Name  string
	Email string
	When  time.Time
}

type commit struct {
	// Hash of the commit object.
	Hash hash
	// Author is the original author of the commit.
	Author signature
	// Committer is the one performing the commit, might be different from
	// Author.
	Committer signature
}

// Replace patterns in a string with their values from information in a git commit.
// the %token% style of tokens is deprecated, use {token}.
// {git.hash}               -> the full hex encoded commit id.
// {git.hash.short}         -> the first 7 characters of the hex encoded commit id.
// {git.commit.year}        -> year of the commit, in UTC.
// {git.commit.month}       -> month of the commit, in UTC.
// {git.commit.day}         -> day of the commit, in UTC.
// {git.commit.seconds}     -> seconds into the day of the commit, in UTC (0 to 86399). with the year, month and day
// it orders commits and, unlike unix, never passes 2147483647.
// {git.commit.seconds.short} -> seconds, zero padded to 5 digits (00000 to 86399). the fixed width allows following the
// day with it, e.g.) {git.commit.day}{git.commit.seconds.short} is a number that sorts by commit time.
// {git.commit.unix}        -> Unix of the commit.
// {git.commit.unix.milli}  -> UnixMilli of the commit.
// {git.commit.unix.micro}  -> UnixMicro of the commit.
// {eg.git.canonical.uri}   -> the uri of the repository that eg considers to be the canonical uri.
func (c commit) StringReplace(pattern string) string {
	patternx.Warn(pattern)
	// the date is taken in UTC so it does not depend on the committer's timezone and stays consistent with unix.
	cwhen := c.Committer.When.UTC()

	s := strings.ReplaceAll(pattern, "%git.hash%", c.Hash.String())
	s = strings.ReplaceAll(s, "{git.hash}", c.Hash.String())
	s = strings.ReplaceAll(s, "%git.hash.short%", fmt.Sprintf("%7s", c.Hash))
	s = strings.ReplaceAll(s, "{git.hash.short}", fmt.Sprintf("%7s", c.Hash))
	s = strings.ReplaceAll(s, "%git.commit.year%", strconv.Itoa(cwhen.Year()))
	s = strings.ReplaceAll(s, "{git.commit.year}", strconv.Itoa(cwhen.Year()))
	s = strings.ReplaceAll(s, "%git.commit.month%", strconv.Itoa(int(cwhen.Month())))
	s = strings.ReplaceAll(s, "{git.commit.month}", strconv.Itoa(int(cwhen.Month())))
	s = strings.ReplaceAll(s, "%git.commit.day%", strconv.Itoa(cwhen.Day()))
	s = strings.ReplaceAll(s, "{git.commit.day}", strconv.Itoa(cwhen.Day()))
	daysecs := cwhen.Hour()*3600 + cwhen.Minute()*60 + cwhen.Second()
	s = strings.ReplaceAll(s, "{git.commit.seconds}", strconv.Itoa(daysecs))
	s = strings.ReplaceAll(s, "{git.commit.seconds.short}", fmt.Sprintf("%05d", daysecs))
	s = strings.ReplaceAll(s, "%git.commit.unix%", strconv.Itoa(int(cwhen.Unix())))
	s = strings.ReplaceAll(s, "{git.commit.unix}", strconv.Itoa(int(cwhen.Unix())))
	s = strings.ReplaceAll(s, "%git.commit.unix.milli%", strconv.Itoa(int(cwhen.UnixMilli())))
	s = strings.ReplaceAll(s, "{git.commit.unix.milli}", strconv.Itoa(int(cwhen.UnixMilli())))
	s = strings.ReplaceAll(s, "%git.commit.unix.micro%", strconv.Itoa(int(cwhen.UnixMicro())))
	s = strings.ReplaceAll(s, "{git.commit.unix.micro}", strconv.Itoa(int(cwhen.UnixMicro())))
	s = strings.ReplaceAll(s, "%eg.git.canonical.uri%", EnvCanonicalURI())
	s = strings.ReplaceAll(s, "{eg.git.canonical.uri}", EnvCanonicalURI())

	return s
}

// completely clean any git substitutions from the pattern.
func PatternClean(pattern string) string {
	patternx.Warn(pattern)
	s := strings.ReplaceAll(pattern, ".%", "%")
	s = strings.ReplaceAll(s, "%.", "%")
	s = strings.ReplaceAll(s, ".{", "{")
	s = strings.ReplaceAll(s, "}.", "}")
	s = strings.ReplaceAll(s, "%git.hash%", "")
	s = strings.ReplaceAll(s, "{git.hash}", "")
	s = strings.ReplaceAll(s, "%git.hash.short%", "")
	s = strings.ReplaceAll(s, "{git.hash.short}", "")
	s = strings.ReplaceAll(s, "%git.commit.year%", "")
	s = strings.ReplaceAll(s, "{git.commit.year}", "")
	s = strings.ReplaceAll(s, "%git.commit.month%", "")
	s = strings.ReplaceAll(s, "{git.commit.month}", "")
	s = strings.ReplaceAll(s, "%git.commit.day%", "")
	s = strings.ReplaceAll(s, "{git.commit.day}", "")
	s = strings.ReplaceAll(s, "{git.commit.seconds}", "")
	s = strings.ReplaceAll(s, "{git.commit.seconds.short}", "")
	s = strings.ReplaceAll(s, "%git.commit.unix%", "")
	s = strings.ReplaceAll(s, "{git.commit.unix}", "")
	s = strings.ReplaceAll(s, "%git.commit.unix.milli%", "")
	s = strings.ReplaceAll(s, "{git.commit.unix.milli}", "")
	s = strings.ReplaceAll(s, "%git.commit.unix.micro%", "")
	s = strings.ReplaceAll(s, "{git.commit.unix.micro}", "")
	s = strings.ReplaceAll(s, "%eg.git.canonical.uri%", "")
	s = strings.ReplaceAll(s, "{eg.git.canonical.uri}", "")
	return s
}

// substitute values in the provided pattern using the environment variable commit.
// see commit.StringReplace for more details.
func Pattern(s string) string {
	return EnvCommit().StringReplace(s)
}

// substitute values in the provided pattern using the environment variable commit.
//
// Deprecated: use Pattern.
func StringReplace(pattern string) string {
	return Pattern(pattern)
}

func EnvCanonicalURI() string {
	return os.Getenv(_eg.EnvComputeVCS)
}

// retrieve the commit metadata from from the environment.
func EnvCommit() *commit {
	return &commit{
		Hash: nhash(os.Getenv(_eg.EnvGitHeadCommit)),
		Committer: signature{
			Name:  os.Getenv(_eg.EnvGitHeadCommitAuthor),
			Email: os.Getenv(_eg.EnvGitHeadCommitEmail),
			When:  errorsx.Zero(time.Parse(time.RFC3339, os.Getenv(_eg.EnvGitHeadCommitTimestamp))),
		},
	}
}

// determine the commit hash for the given name.
func Commitish(ctx context.Context, treeish string) string {
	return ffigit.Commitish(ctx, treeish)
}

// clone a git repository from the given uri, remote and treeish into dir, the commit is preferred.
// dir is required, a relative dir is relative to the working directory.
//
// the clone always happens, EG_UNSAFE_GIT_CLONE_ENABLED=false, which local runs set to avoid cloning the
// workload's own repository, is ignored. see AutoClone for the clone that honors it.
func Clone(ctx context.Context, dir, uri, remote, commit string) error {
	environ, err := egenv.Build().FromEnviron(os.Environ()...).Var(_eg.EnvUnsafeGitCloneEnabled, true).Environ()
	if err != nil {
		return err
	}

	return clone(ctx, []ffigit.Option{ffigit.Environ(environ...)}, dir, uri, remote, commit)
}

func clone(ctx context.Context, opts []ffigit.Option, dir, uri, remote, commit string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("unable to clone destination not specified: %s", dir)
	}

	if strings.TrimSpace(uri) == "" {
		return fmt.Errorf("unable to clone url not specified: %s", uri)
	}

	if strings.TrimSpace(remote) == "" {
		return fmt.Errorf("unable to clone remote not specified: %s", remote)
	}

	return ffigit.Clone(ctx, dir, uri, remote, commit, opts...)
}

// CloneOp returns an operation that clones a git repository, see Clone.
func CloneOp(dir, uri, remote, commit string) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		return Clone(ctx, dir, uri, remote, commit)
	}
}

// AutoCached returns the directory within the repository cache that holds the clone of uri,
// {cache}/git/{md5(uri)}.git. the name is derived from the uri alone, so every call for the
// same uri, from any module or workload sharing the cache, resolves to the same directory and
// different uris never collide. the name is the one the host uses for its own mirrors of uri.
//
// the directory is only computed, it is not created. see egenv.DirectoryCache for the lifecycle
// of the cache: it survives across workloads on a best effort basis, so never rely on its contents.
func AutoCached(uri string) string {
	return egenv.DirectoryCache("git", fmt.Sprintf("%s.git", md5x.String(uri)))
}

// CachedClone returns an operation that clones a git repository into the repository cache,
// see AutoCached for the location. it is CloneOp without the dir argument: uri, remote and commit
// behave as they do for Clone, and the same validation applies, a blank uri or remote is an error.
//
// because the location is stable for a uri, a later CachedClone, or any other consumer of
// AutoCached(uri), finds the repository where it was left and the clone only has to bring
// it up to date. use CloneOp when the repository needs to live at a specific path, e.g.) the
// working directory.
func CachedClone(uri, remote, commit string) eg.OpFn {
	return CloneOp(AutoCached(uri), uri, remote, commit)
}

// clone the repository specified by the eg environment variables into the working directory.
//
// unlike Clone it honors EG_UNSAFE_GIT_CLONE_ENABLED=false, which local runs set because the
// workload's repository is already present, and skips the clone.
func AutoClone(ctx context.Context, _ eg.Op) error {
	if err := clone(ctx, nil, egenv.DirectoryWorking(), env.String("", _eg.EnvGitHeadURI), env.String("origin", "EG_GIT_HEAD_REMOTE"), env.String("main", _eg.EnvGitHeadCommit, _eg.EnvGitHeadRef)); err != nil {
		return err
	}

	return nil
}

// Archive creates a gzipped tar archive of the git repository at dir, writing it to dest.
// The archive is created from the commit specified by the eg environment.
func Archive(dir, dest string) eg.OpFn {
	return archive(shell.Runtime(), dir, dest)
}

func archive(runtime shell.Command, dir, dest string) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		return shell.Run(
			ctx,
			runtime.Newf("git archive --format=tar.gz -o %s %s", dest, EnvCommit().StringReplace("{git.hash}")).Directory(dir),
		)
	}
}

// AutoArchive archives the repository cloned into the working directory, see AutoClone, to
// {commit}.tar.gz within the workspace directory.
func AutoArchive(ctx context.Context, op eg.Op) error {
	return autoarchive(shell.Runtime())(ctx, op)
}

func autoarchive(runtime shell.Command) eg.OpFn {
	return func(ctx context.Context, op eg.Op) error {
		dir := egenv.DirectoryWorking()
		dest := egenv.DirectoryWorkload(EnvCommit().StringReplace("{git.hash}.tar.gz"))
		return archive(runtime, dir, dest)(ctx, op)
	}
}

// ensures the workspace has not been modified. useful for detecting
// if there have been changes during the run.
func Pristine() eg.OpFn {
	return pristine(shell.Runtime())
}

func pristine(runtime shell.Command) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		log.Println("ensuring git repository has not changed initiated")
		defer log.Println("ensuring git repository has not changed completed")
		return shell.Run(
			ctx,
			runtime.New("git diff-index --name-only HEAD"),
			runtime.New("git diff-index --quiet HEAD"),
		)
	}
}
