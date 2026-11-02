package compute

import (
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
