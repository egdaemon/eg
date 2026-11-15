package compute

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/internal/uuidx"
)

var invalidbranchchars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// defaultbranch is the branch a workload's changes are committed to when the session has none saved and none
// was requested: {username}/eg-{workload}-{the last 4 bytes of the run id}, the default workload has no name so
// it is {username}/eg-{the last 4 bytes of the run id}.
func defaultbranch(workload string, id uuidx.UUID) string {
	clean := func(s string) string {
		return stringsx.DefaultIfBlank(invalidbranchchars.ReplaceAllString(s, "-"), "eg")
	}

	user := clean(userx.CurrentUsername(eg.DefaultUsername))
	if strings.TrimSpace(workload) == "" {
		return fmt.Sprintf("%s/eg-%x", user, id[uuidx.Size-4:])
	}

	return fmt.Sprintf("%s/eg-%s-%x", user, clean(workload), id[uuidx.Size-4:])
}
