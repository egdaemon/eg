//go:build !wasip1

package coreutils

import (
	"github.com/u-root/u-root/pkg/core"
	"github.com/u-root/u-root/pkg/core/find"
	"github.com/u-root/u-root/pkg/core/ls"
)

// u-root's ls package, which find also uses, has no wasip1 implementation, so on wasip1 both go to the next handler.
func init() {
	commandBuilders["find"] = func() core.Command { return find.New() }
	commandBuilders["ls"] = func() core.Command { return ls.New() }
}
