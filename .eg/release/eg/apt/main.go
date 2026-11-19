// Package main builds and publishes the signed apt repository for eg.
//
// the .deb files are not copied anywhere, they stay on the github releases where the release workloads promote them.
// only the index (dists/, InRelease, Release.gpg and the public key) is generated here and published to the gh-pages
// branch, whose Packages file points every package at its github release download url.
//
// users consume it with:
//
//	curl -fsSL https://<owner>.github.io/<repo>/key.asc | sudo tee /etc/apt/keyrings/eg.asc
//	echo "deb [signed-by=/etc/apt/keyrings/eg.asc arch=amd64] https://<owner>.github.io/<repo> stable main" | sudo tee /etc/apt/sources.list.d/eg.list
package main

import (
	"context"
	"log"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
	"github.com/egdaemon/eg/runtime/x/wasi/eggpg"
)

const (
	suite        = "stable"
	component    = "main"
	architecture = "amd64"
)

// the https url of the repository, derived from the canonical uri the same way eggithub.DownloadURL does.
// git@github.com:egdaemon/eg.git -> https://github.com/egdaemon/eg.git
func remote() string {
	canon := eggit.EnvCanonicalURI()
	canon = strings.ReplaceAll(canon, ":", "/")
	return strings.ReplaceAll(canon, "git@", "https://")
}

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile | log.LUTC)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	var (
		// eggpg.Options() takes GNUPGHOME from the environment, which inside the container is the
		// host-provided /eg.mnt/.gnupg (not writable by us: "unsafe ownership", "Permission denied").
		// pin the isolated per-container home instead.
		gpgoptions = eggpg.Options().IgnoreLocalGNU().Home("/home/egd/.gnupg")

		// the directories of the repository, everything under site is published.
		work   = egenv.DirectoryWorkload("apt")
		pool   = filepath.Join(work, "dl")
		site   = filepath.Join(work, "site")
		dist   = filepath.Join(site, "dists", suite)
		binary = filepath.Join(dist, component, "binary-"+architecture)

		// one runtime for every command: the process environment, the github token for gh and git, the
		// isolated gnupg home for gpg, and the repository layout the commands below refer to.
		rt = shell.Env().
			EnvironFrom(eggithub.Env()...).
			EnvironFrom(eggpg.Env(gpgoptions...)...).
			Environ("APT_SUITE", suite).
			Environ("APT_COMPONENT", component).
			Environ("APT_ARCH", architecture).
			Environ("APT_POOL", pool).
			Environ("APT_SITE", site).
			Environ("APT_DIST", dist).
			Environ("APT_BINARY", binary).
			Environ("APT_REMOTE", remote()).
			Directory(work)
		// gh resolves {owner}/{repo} from the git remote of the cloned repository, and work does not exist yet.
		repo = rt.Directory(egenv.DirectoryWorking())
	)

	// mirror every amd64 deb attached to a published release under dl/github.com/<owner>/<repo>/releases/download/<tag>/<deb>,
	// fails when there are none.
	mirror := shell.Op(
		repo.New("mkdir -p ${APT_BINARY} ${APT_SITE}"),
		repo.New(`gh api repos/{owner}/{repo}/releases --paginate --jq '.[] | select(.draft | not) | .assets[] | select(.name | endswith("_" + env.APT_ARCH + ".deb")) | .browser_download_url' | wget -q -x -i - -P ${APT_POOL}`),
	)

	// private repository mode: the debs are uploaded to the eg hosted apt repository named by EG_APT_REPOSITORY,
	// which builds, signs and serves the index itself, instead of publishing the gh-pages repository below.
	// the eg in the workload must be authorized for the account (repository_modify), see eg authz ci token.
	if private := egenv.String("", "EG_APT_REPOSITORY"); private != "" {
		prt := rt.Environ("EG_APT_REPOSITORY", private).Directory(pool)
		if err := eg.Perform(
			ctx,
			eggit.AutoClone,
			mirror,
			shell.Op(
				prt.New("find . -name '*.deb' -print0 | xargs -0 --no-run-if-empty eg apt upload ${EG_APT_REPOSITORY}"),
				prt.New("eg apt publish ${EG_APT_REPOSITORY}"),
			),
		); err != nil {
			log.Fatalln(err)
		}
		return
	}

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eggpg.Seed(gpgoptions...),
		mirror,
		shell.Op(
			// the path prefix makes every Filename the release download url apt fetches the package from.
			rt.New("dpkg-scanpackages --multiversion github.com /dev/null https:// > ${APT_BINARY}/Packages").Directory(pool),
			rt.New("gzip -9 -k -f ${APT_BINARY}/Packages"),
			rt.New("apt-ftparchive -o APT::FTPArchive::Release::Origin=egdaemon -o APT::FTPArchive::Release::Label=eg -o APT::FTPArchive::Release::Suite=${APT_SUITE} -o APT::FTPArchive::Release::Codename=${APT_SUITE} -o APT::FTPArchive::Release::Architectures=${APT_ARCH} -o APT::FTPArchive::Release::Components=${APT_COMPONENT} release ${APT_DIST} > ${APT_DIST}/Release"),
			rt.New("gpg --batch --yes --local-user ${EG_GPG_KEYRING_EMAIL} --clearsign --output ${APT_DIST}/InRelease ${APT_DIST}/Release"),
			rt.New("gpg --batch --yes --local-user ${EG_GPG_KEYRING_EMAIL} --armor --detach-sign --output ${APT_DIST}/Release.gpg ${APT_DIST}/Release"),
			// the public half generated by eggpg.Seed, no secret material is read.
			rt.New("cp ${EG_GPG_KEYRING_HOME}/public.asc ${APT_SITE}/key.asc"),
			rt.New("touch ${APT_SITE}/.nojekyll"),
			// the branch is entirely generated from the releases, so history has no value, it is replaced on every run.
			rt.New("gh auth setup-git"),
			rt.New("git init -q -b gh-pages").Directory(site),
			rt.New("git add -A").Directory(site),
			rt.New(`git -c user.name="${EG_GPG_KEYRING_NAME}" -c user.email="${EG_GPG_KEYRING_EMAIL}" commit -q -m 'apt repository'`).Directory(site),
			rt.New("git push --force ${APT_REMOTE} gh-pages").Directory(site),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
