package lan

import (
	"context"
	"database/sql"
	"time"

	"github.com/egdaemon/eg/internal/libp2px"
)

// Discover scans the network using find and remembers what is found.
func Discover(ctx context.Context, db *sql.DB, find libp2px.MDNSLookup, timeout time.Duration) (err error) {
	found := find(ctx, timeout)

	// remember each system as it is found so a scan that is cut short keeps what it saw.
	for info := range found.Each(ctx) {
		if err = Remember(ctx, db, info); err != nil {
			return err
		}
	}

	return found.Err()
}
