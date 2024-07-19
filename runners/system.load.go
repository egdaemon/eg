package runners

import (
	"context"
	"database/sql"
	"time"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/timex"
	"github.com/gofrs/uuid"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

func systemload(ctx context.Context, analytics *sql.DB) {
	report := func(do func(ctx context.Context, analytics *sql.DB) error) {
		errorsx.Log(do(ctx, analytics))
	}

	go report(systemloadcpu)
	go report(systemloadmemory)
}

func systemloadcpu(ctx context.Context, analytics *sql.DB) error {
	if _, err := analytics.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS 'metrics.eg.cpu' (id UUID PRIMARY KEY, name TEXT NOT NULL, ts TIMESTAMP NOT NULL, load FLOAT4 NOT NULL)"); err != nil {
		return err
	}

	return timex.NowAndEvery(ctx, 5*time.Second, func(ctx context.Context) error {
		var (
			err   error
			loads []float64
		)

		if loads, err = cpu.PercentWithContext(ctx, 0, false); err != nil {
			return errorsx.Wrap(err, "unable to retrieve compute")
		}

		load := slicesx.FirstOrZero(loads...)

		if err := analytics.QueryRowContext(ctx, "INSERT INTO 'metrics.eg.cpu' (id, name, ts, load) VALUES (?, 'compute', ?, ?) RETURNING load", uuid.Must(uuid.NewV7()).String(), time.Now().UTC(), load).Scan(&load); err != nil {
			return err
		}

		debugx.Println("metrics.eg.compute", load)
		return nil
	})
}

func systemloadmemory(ctx context.Context, analytics *sql.DB) error {
	const query = "INSERT INTO 'metrics.eg.memory' (id, name, ts, percent, unused, used, cached, total) VALUES (?, 'memory', ?, ?, ?, ?, ?, ?) RETURNING percent"
	if _, err := analytics.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS 'metrics.eg.memory' (id UUID PRIMARY KEY, name TEXT NOT NULL, ts TIMESTAMP NOT NULL, percent FLOAT4 NOT NULL, unused INT8 NOT NULL, used INT8 NOT NULL, cached INT8 NOT NULL, total INT8 NOT NULL)"); err != nil {
		return err
	}
	// unused, used, cached, total
	return timex.NowAndEvery(ctx, 5*time.Second, func(ctx context.Context) error {
		var (
			err     error
			usage   *mem.VirtualMemoryStat
			percent float64
		)

		if usage, err = mem.VirtualMemoryWithContext(ctx); err != nil {
			return errorsx.Wrap(err, "unable to retrieve memory usage")
		}

		if err := analytics.QueryRowContext(ctx, query, uuid.Must(uuid.NewV7()).String(), time.Now().UTC(), usage.UsedPercent, usage.Free, usage.Used, usage.Cached, usage.Total).Scan(&percent); err != nil {
			return err
		}

		debugx.Println("metrics.eg.memory.percent", usage.UsedPercent)
		debugx.Println("metrics.eg.memory.free", usage.Free)
		debugx.Println("metrics.eg.memory.used", usage.Used)
		debugx.Println("metrics.eg.memory.cached", usage.Cached)
		debugx.Println("metrics.eg.memory.total", usage.Total)
		return nil
	})
}
