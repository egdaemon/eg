package egautogentest

import (
	"fmt"
)

func violationrecord(rule int, line int, reason string) string {
	return fmt.Sprintf(`{"rule": %d, "line": %d, "reason": %q}`, rule, line, reason)
}
