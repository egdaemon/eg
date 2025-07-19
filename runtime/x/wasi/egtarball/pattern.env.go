package egtarball

import (
	"fmt"
	"os"
	"regexp"
)

func EnvPattern(pattern string, env func(string) string) string {
	formatenv := func(s string) string {
		re := regexp.MustCompile(`%%env\.([^%]+)%%`)
		return re.ReplaceAllStringFunc(s, func(match string) string {
			submatch := re.FindStringSubmatch(match)
			if len(submatch) <= 1 {
				return match
			}

			return os.Expand(fmt.Sprintf("${%s}", submatch[1]), env)
		})
	}
	return formatenv(pattern)
}
