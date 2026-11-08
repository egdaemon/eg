package compute

import (
	"strings"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/workspaces"
)

// baremetalenv appends the inherited environment and then the directories of the workspace to the environment of
// the workload. later entries win, so the directories of the workspace take precedence over the inherited ones.
// a nested workload inherits the directories of the workload that launched it, which are not available to it.
func baremetalenv(b *envx.Builder, ws workspaces.Context, inherited ...string) *envx.Builder {
	return b.FromEnviron(
		inherited...,
	).FromEnviron(
		parentvcs(inherited...)...,
	).Var(
		eg.EnvComputeWorkloadDirectory, ws.Root,
	).Var(
		eg.EnvComputeRuntimeDirectory, ws.RuntimeDir,
	).Var(
		eg.EnvComputeCacheDirectory, ws.CacheDir,
	).Var(
		eg.EnvComputeWorkspaceDirectory, ws.WorkspaceDir,
	).Var(
		eg.EnvComputeWorkingDirectory, ws.WorkingDir,
	).Var(
		eg.EnvComputeDefaultGroup, defaultgroup(),
	)
}

// parentvcs are the vcs entries of the inherited environment. a nested workload is cloned from the path of the
// workload that launched it, which derives a vcs that is not the one of the repository, so those of the parent win.
// blank values are dropped, they would erase the one derived from the workspace.
func parentvcs(inherited ...string) (vcs []string) {
	for _, e := range inherited {
		switch k, v, _ := strings.Cut(e, "="); k {
		case eg.EnvComputeVCS, eg.EnvGitHeadVCS, eg.EnvGitBaseVCS:
			if v != "" {
				vcs = append(vcs, e)
			}
		}
	}

	return vcs
}

// moduleenv appends the vcs of the parent and then the directories of the container to the environment provided to
// the module. nothing else is inherited, the directories of the parent are not available to the module and are
// replaced by those of the container.
func moduleenv(b *envx.Builder, inherited ...string) *envx.Builder {
	b = b.FromEnviron(parentvcs(inherited...)...)

	return b.Var(
		eg.EnvComputeWorkloadDirectory, eg.DefaultRootDirectory(),
	).Var(
		eg.EnvComputeWorkingDirectory, eg.DefaultWorkingDirectory(),
	).Var(
		eg.EnvComputeCacheDirectory, eg.DefaultCacheDirectory(),
	).Var(
		eg.EnvComputeRuntimeDirectory, eg.DefaultRuntimeDirectory(),
	).Var(
		eg.EnvComputeWorkspaceDirectory, eg.DefaultWorkspaceDirectory(),
	)
}

// hostmoduleenv is the environment provided to a module that runs its commands on the host. it is moduleenv with
// the directories of the workspace in place of those of the container.
func hostmoduleenv(b *envx.Builder, ws workspaces.Context, inherited ...string) *envx.Builder {
	return moduleenv(b, inherited...).Var(
		eg.EnvComputeWorkloadDirectory, ws.Root,
	).Var(
		eg.EnvComputeRuntimeDirectory, ws.RuntimeDir,
	).Var(
		eg.EnvComputeCacheDirectory, ws.CacheDir,
	).Var(
		eg.EnvComputeWorkspaceDirectory, ws.WorkspaceDir,
	).Var(
		eg.EnvComputeWorkingDirectory, ws.WorkingDir,
	)
}
