package cmdidentity

import (
	"strings"

	"github.com/egdaemon/eg/internal/stringsx"
)

// dashed joins the present values with " - ".
func dashed(values ...string) string {
	present := make([]string, 0, len(values))
	for _, v := range values {
		if stringsx.Present(v) {
			present = append(present, v)
		}
	}

	return strings.Join(present, " - ")
}
