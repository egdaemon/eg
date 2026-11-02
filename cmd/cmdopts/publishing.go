package cmdopts

import "errors"

// Publishing are the flags deciding how far a workload that releases software goes, see egenv.Draft and egenv.Release.
// workloads build into the workspace by default and push nothing.
type Publishing struct {
	Draft   bool `flag:"" name:"draft" negatable:"" help:"allow the workload to upload its artifacts to a draft release" default:"false" env:"${env_eg_workload_draft}"`
	Release bool `flag:"" name:"release" negatable:"" help:"allow the workload to publish the release, requires --draft" default:"false" env:"${env_eg_workload_release}"`
}

// Validate publishing builds on the draft, so --no-draft implies --no-release.
func (t Publishing) Validate() error {
	if t.Release && !t.Draft {
		return errors.New("--release requires --draft")
	}

	return nil
}
