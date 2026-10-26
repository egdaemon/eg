// guest used by the interp tests: reports coverage batches over the analytics socket
// from inside the wasi runtime, one fresh connection per report, as RecordCoverage does.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoverage"
	"github.com/egdaemon/eg/runtime/x/wasi/egdb"
)

// SESSIONS_MODE selects what each session does:
//
//	dial   - dial the socket and close it, nothing else.
//	ping   - egdb.Open (connect + ping), a trivial query, and close.
//	report - fficoverage.Report of a full batch. (default)
func session(mode string, i, size int) error {
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	switch mode {
	case "dial":
		c, err := egunsafe.DialAnalyticsSocket(ctx, "unix", egunsafe.RuntimeDirectory(eg.SocketAnalytics))
		if err != nil {
			return err
		}
		return c.Close()
	case "ping":
		db, err := egdb.Open(ctx)
		if err != nil {
			return err
		}
		defer db.Close()

		var n int
		return db.QueryRowContext(ctx, "SELECT 1").Scan(&n)
	default:
		batch := make([]*fficoverage.Coverage, 0, size)
		for j := range size {
			batch = append(batch, &fficoverage.Coverage{
				Path:       "p" + strconv.Itoa(i) + ".go",
				Fnname:     fmt.Sprintf("Fn%d", j),
				Hits:       int64(j),
				Statements: 50,
				Branches:   50,
			})
		}

		return fficoverage.Report(ctx, batch...)
	}
}

func main() {
	const size = 128

	reports := envx.Int(12, "SESSIONS_COUNT")
	mode := envx.String("report", "SESSIONS_MODE")

	for i := range reports {
		if err := session(mode, i, size); err != nil {
			log.Printf("%s %d: %v", mode, i, err)
			os.Exit(1)
		}
	}

	log.Println(mode, "completed", reports)
}
