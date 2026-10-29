// Package main analyzes what the GitHub token of the workload is allowed to do, i.e.) the installation of the
// GitHub App when the runner issued the token. each endpoint is requested once and reported with its status and the
// permission GitHub says the endpoint requires, nothing is changed in the repository and no token is printed.
//
// a 2xx status means the token holds the permission. 403 is the permission missing, 404 is the permission missing
// or the repository is not part of the installation.
//
// the workload fails when the token cannot open pull requests, the other probes are informational.
// run it with: eg compute upload tests/github.probe
package main

import (
	"context"
	"log"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffigit"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// probe gives gh a repository to resolve {owner} and {repo} from. it only reads the url of the remote, so an empty
// repository pointing at the head of the workload is enough, no clone or network needed.
func probe(ctx context.Context, op eg.Op) error {
	runtime := shell.Runtime().Environ(_eg.EnvGithubToken, ffigit.Bearer())
	uri := env.String("", _eg.EnvGitHeadURI)

	return shell.Run(
		ctx,
		runtime.Environ("EG_PROBE_URI", uri).New(`git init -q && (git remote add origin "${EG_PROBE_URI}" || git remote set-url origin "${EG_PROBE_URI}")`),
		runtime.New("gh auth status").Lenient(true),
		// only an installation token can list the repositories of its installation.
		runtime.New(`gh api 'installation/repositories?per_page=100' --jq '.repositories[] | "\(.full_name) \(.permissions)"'`).Lenient(true),
		runtime.New("gh api -i repos/{owner}/{repo} | grep -iE '^(HTTP/|x-accepted-github-permissions:)'").Lenient(true),
		runtime.New("gh api -i repos/{owner}/{repo}/contents | grep -iE '^(HTTP/|x-accepted-github-permissions:)'").Lenient(true),
		runtime.New("gh api -i 'repos/{owner}/{repo}/pulls?per_page=1' | grep -iE '^(HTTP/|x-accepted-github-permissions:)'").Lenient(true),
		runtime.New("gh api -i 'repos/{owner}/{repo}/issues?per_page=1' | grep -iE '^(HTTP/|x-accepted-github-permissions:)'").Lenient(true),
		runtime.New("gh api -i 'repos/{owner}/{repo}/actions/workflows?per_page=1' | grep -iE '^(HTTP/|x-accepted-github-permissions:)'").Lenient(true),
		// opening a pull request has no dry run, so it asks for one from a branch that does not exist. a token
		// allowed to open pull requests is rejected for the head with a 422 and nothing is created.
		runtime.New(`out=$(gh api -i --method POST repos/{owner}/{repo}/pulls -f title=probe -f head=eg-probe/does-not-exist -f base=main 2>&1)
echo "$out" | grep -iE '^(HTTP/|x-accepted-github-permissions:)'
echo "$out" | grep -q '^HTTP/[0-9.]* 422'`),
	)
}

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		probe,
	)

	if err != nil {
		log.Fatalln(err)
	}
}
