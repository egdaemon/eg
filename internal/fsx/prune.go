package fsx

import (
	"context"
	"errors"

	"github.com/egdaemon/eg/internal/iterx"
)

// Prune removes every path yielded by s, along with anything within them, including read-only trees, see RemoveAll. all paths are attempted,
// the errors from removing them and from s are joined.
func Prune(ctx context.Context, s iterx.Seq[string]) error {
	var failed []error

	for path := range s.Each(ctx) {
		failed = append(failed, RemoveAll(path))
	}

	return errors.Join(append(failed, s.Err())...)
}
