package runners

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

// newLocalSourceRepo creates a local (no-network) git repo containing the
// eg/compile package's example.1 fixture as its .eg module, so
// compileWorkload can clone+compile it without a VCS auth token or network
// access to a real forge.
func newLocalSourceRepo(t *testing.T) (uri, ref string) {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, fsx.CloneTree(
		t.Context(),
		filepath.Join(dir, ".eg"),
		filepath.Join("example.1", ".eg"),
		os.DirFS(filepath.Join("..", "compile", ".fixtures")),
	))

	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	w, err := repo.Worktree()
	require.NoError(t, err)

	_, err = w.Add(".")
	require.NoError(t, err)

	_, err = w.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "eg", Email: "eg@example.com"},
	})
	require.NoError(t, err)

	head, err := repo.Head()
	require.NoError(t, err)

	return dir, head.Name().Short()
}

func TestCompileWorkload(t *testing.T) {
	t.Run("compiles a local source ref and hands it directly to rundirs.Queued, bypassing Downloading", func(t *testing.T) {
		uri, ref := newLocalSourceRepo(t)

		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		uid := uuid.Must(uuid.NewV7())
		req := EnqueuedDequeueResponse{
			Enqueued: &Enqueued{
				VcsUri:    uri,
				VcsCommit: ref,
				Cores:     1,
			},
		}
		encoded, err := json.Marshal(&req)
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(uid, "metadata.json", bytes.NewReader(encoded)))
		require.NoError(t, compiledirs.Enqueue(uid))

		dir, err := compiledirs.Dequeue()
		require.NoError(t, err)

		require.NoError(t, compileWorkload(t.Context(), http.DefaultClient, dir, rundirs))

		// bypasses rundirs.Downloading entirely.
		dentries, err := os.ReadDir(rundirs.Downloading)
		require.NoError(t, err)
		require.Empty(t, dentries)

		qentries, err := os.ReadDir(rundirs.Queued)
		require.NoError(t, err)
		require.Len(t, qentries, 1)
		require.Equal(t, Queued().Dirname(uid), qentries[0].Name())

		target := filepath.Join(rundirs.Queued, qentries[0].Name())
		require.FileExists(t, filepath.Join(target, "archive.tar.gz"))

		mencoded, err := os.ReadFile(filepath.Join(target, "metadata.json"))
		require.NoError(t, err)

		var resp EnqueuedDequeueResponse
		require.NoError(t, json.Unmarshal(mencoded, &resp))
		require.Equal(t, uid.String(), resp.Enqueued.Id)
		require.NotEmpty(t, resp.Enqueued.Entry)
		require.Equal(t, ref, resp.Enqueued.VcsCommit)

		// clone artifacts are cleaned up, not left behind in the run directory.
		require.NoDirExists(t, filepath.Join(target, "src"))

		// the incoming request environment is overwritten by the outgoing
		// workload environment once cloning is done, so any credentials it
		// carried don't linger.
		outgoing, err := os.ReadFile(filepath.Join(target, eg.EnvironFile))
		require.NoError(t, err)
		require.NotContains(t, string(outgoing), "EG_GIT_AUTH_ACCESS_TOKEN")

		// the compile-side job directory is gone -- it was renamed, not copied.
		require.NoDirExists(t, dir)
	})

	t.Run("failed compiles are discarded and never reach rundirs.Queued", func(t *testing.T) {
		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		uid := uuid.Must(uuid.NewV7())
		req := EnqueuedDequeueResponse{Enqueued: &Enqueued{VcsUri: filepath.Join(t.TempDir(), "does-not-exist")}}
		encoded, err := json.Marshal(&req)
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(uid, "metadata.json", bytes.NewReader(encoded)))
		require.NoError(t, compiledirs.Enqueue(uid))

		dir, err := compiledirs.Dequeue()
		require.NoError(t, err)

		require.Error(t, compileWorkload(t.Context(), http.DefaultClient, dir, rundirs))

		qentries, err := os.ReadDir(rundirs.Queued)
		require.NoError(t, err)
		require.Empty(t, qentries)
	})
}
