package llamacpp

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"eg/compute/errorsx"
	"eg/compute/maintainer"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egccache"
	"github.com/egdaemon/eg/runtime/x/wasi/egdebuild"
)

//go:embed .debskel
var debskel embed.FS

const (
	container = "eg.deb.llamacpp"
	tag       = "b9693" // upstream git tag; llama.cpp's "b<number>" releases aren't usable as a debian version (must start with a digit).
	version   = "9693"  // tag without the leading 'b'.
)

var gcfg egdebuild.Config

func init() {
	egccache.CacheDirectory()
	c := eggit.EnvCommit()
	gcfg = egdebuild.New(
		"llama.cpp",
		"",
		egenv.CacheDirectory("llamacpp"),
		egdebuild.Option.Maintainer(maintainer.Name, maintainer.Email),
		egdebuild.Option.SigningKeyID(maintainer.GPGFingerprint),
		egdebuild.Option.ChangeLogDate(c.Committer.When),
		egdebuild.Option.Version(fmt.Sprintf("%s.:autopatch:", version)),
		egdebuild.Option.Description("llama.cpp", "LLM inference in C/C++ (llama-server, llama-cli)"),
		egdebuild.Option.Debian(errorsx.Must(fs.Sub(debskel, ".debskel"))),
		egdebuild.Option.DependsBuild("rsync", "curl", "tree", "ca-certificates", "cmake", "ninja-build", "git", "libssl-dev", "libvulkan-dev", "glslc", "spirv-headers"),
		egdebuild.Option.Depends("libvulkan1", "mesa-vulkan-drivers"), // vulkan loader + ICDs for the Vulkan backend
		egdebuild.Option.Envvar("PACKAGE_VERSION", version),
		egdebuild.Option.Envvar("LLAMACPP_TAG", tag),
		egdebuild.Option.Envvar("GIT_COMMIT_HASH", c.Hash.String()),
	)
}

func Prepare(ctx context.Context, o eg.Op) error {
	sruntime := shell.Runtime().Directory(egenv.CacheDirectory())
	return eg.Parallel(
		shell.Op(
			sruntime.Newf("test -d llamacpp || git clone -b %s --depth 1 https://github.com/ggml-org/llama.cpp.git llamacpp", tag),
			sruntime.New("md5sum llamacpp/include/llama.h"),
			sruntime.New("echo \"9a9cf0a56defc5b6d4f8ba4efdbfe15f  llamacpp/include/llama.h\" > llamacpp.md5"),
			sruntime.New("md5sum -c llamacpp.md5"),
		),
		egdebuild.Prepare(Runner(), errorsx.Must(fs.Sub(debskel, ".debskel"))),
	)(ctx, o)
}

// container for this package.
func Runner() eg.ContainerRunner {
	return eg.Container(container)
}

func Build(ctx context.Context, o eg.Op) error {
	return eg.Parallel(
		egdebuild.Build(gcfg, egdebuild.Option.Distro("noble"), egdebuild.Option.NoLint()),
		egdebuild.Build(gcfg, egdebuild.Option.Distro("questing"), egdebuild.Option.NoLint()),
		egdebuild.Build(gcfg, egdebuild.Option.Distro(egdebuild.UbuntuLatestCodename), egdebuild.Option.NoLint()),
	)(ctx, o)
}

func Upload(ctx context.Context, o eg.Op) error {
	return egdebuild.UploadDPut(gcfg, errorsx.Must(fs.Sub(debskel, ".debskel")), egdebuild.Option.Timeout(20*time.Minute))(ctx, o)
}
