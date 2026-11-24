package runners

import (
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/stringsx"
)

// ContainerfileName the name of the file nearest-lookups search for.
const ContainerfileName = "Containerfile"

// ArchiveContainerfile the path, relative to the root of a workload's archive, of the Containerfile the workload's
// container is built from. shared by the uploading side, which packs it, and the runner, which builds it.
const ArchiveContainerfile = ".eg/" + ContainerfileName

// LocateContainerfile the Containerfile a workload's container is built from: the explicit one when provided,
// otherwise the nearest one from workloaddir up to and including root. an explicit path is relative to repo
// unless it is absolute and must exist. an empty path with no error means no Containerfile was found.
// the image is the name to build the Containerfile as: the default container's, eg.WorkingDirectory, when none
// was found, otherwise one unique to the workload, derived from name.
func LocateContainerfile(repo, root, workloaddir, explicit, name string, builtin bool) (path string, image string, err error) {
	if builtin {
		return "", eg.WorkingDirectory, nil
	}

	if explicit != "" {
		path = explicit
		if !filepath.IsAbs(path) {
			path = filepath.Join(repo, path)
		}

		if !fsx.FileExists(path) {
			return "", "", errorsx.Errorf("unable to use the containerfile %s", path)
		}
	} else {
		path = fsx.LocateWithinUntil(workloaddir, root, ContainerfileName)
	}

	if path == "" {
		return "", eg.WorkingDirectory, nil
	}

	return path, stringsx.Join(".", eg.WorkingDirectory, strings.ReplaceAll(stringsx.First(name, "workload"), string(filepath.Separator), ".")), nil
}
