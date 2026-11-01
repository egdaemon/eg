// Package patternx supports the substitution of tokens within patterns. tokens are written {name},
// the %name% style is deprecated.
package patternx

import (
	"log"
	"path"
	"slices"
	"strings"
	"sync"
)

var (
	warned sync.Map
	// the namespaces of the tokens, anything else containing a percent sign is a plain string.
	legacy = []string{"*%git.*%*", "*%eg.*%*", "*%fn.*%*"}
)

// Warn logs, once per pattern, when the pattern uses the deprecated %name% style of tokens.
func Warn(pattern string) {
	// path.Match's * does not cross slashes.
	normalized := strings.ReplaceAll(pattern, "/", "_")
	if !slices.ContainsFunc(legacy, func(glob string) bool {
		ok, _ := path.Match(glob, normalized)
		return ok
	}) {
		return
	}

	if _, seen := warned.LoadOrStore(pattern, struct{}{}); seen {
		return
	}

	log.Printf("pattern %s uses the deprecated %%token%% style, use {token}\n", pattern)
}
