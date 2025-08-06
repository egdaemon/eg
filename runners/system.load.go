package runners

import (
	"context"
	"database/sql"
	"time"

	"github.com/egdaemon/eg/internal/contextx"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/timex"
	"github.com/gofrs/uuid/v5"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	psnet "github.com/shirou/gopsutil/v4/net"
)

func BackgroundSystemLoad(ctx context.Context, analytics *sql.DB) {
	report := func(do func(ctx context.Context, analytics *sql.DB) error) {
		errorsx.Log(
			contextx.IgnoreCancelled(do(ctx, analytics)),
		)
	}

	go report(systemcpu)
	go report(systemmemory)
	go report(systemdisk)
	go report(systemnet)
}

func SampleSystemLoad(ctx context.Context, analytics *sql.DB) error {
	return errorsx.Compact(
		samplecompute(ctx, analytics),
		samplememory(ctx, analytics),
		sampledisk(ctx, analytics),
		samplenet(ctx, analytics),
	)
}

func systemdisk(ctx context.Context, analytics *sql.DB) error {
	if _, err := analytics.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS 'eg.metrics.disk' (id UUID PRIMARY KEY, name TEXT NOT NULL, path TEXT NOT NULL, ts TIMESTAMP NOT NULL, percent FLOAT4 NOT NULL)"); err != nil {
		return err
	}

	return timex.NowAndEvery(ctx, 5*time.Second, func(ctx context.Context) error {
		return sampledisk(ctx, analytics)
	})
}

func sampledisk(ctx context.Context, analytics *sql.DB) error {
	var (
		err   error
		usage *disk.UsageStat
	)

	if usage, err = disk.UsageWithContext(ctx, "/"); err != nil {
		return errorsx.Wrap(err, "unable to retrieve disk")
	}

	if err := analytics.QueryRowContext(ctx, "INSERT INTO 'eg.metrics.disk' (id, name, ts, path, percent) VALUES (?, 'disk', ?, ?, ?) RETURNING percent", uuid.Must(uuid.NewV7()).String(), time.Now().UTC(), usage.Path, usage.UsedPercent).Scan(&usage.UsedPercent); err != nil {
		return err
	}

	debugx.Println("eg.metrics.disk", usage.Path, usage.UsedPercent)

	return nil
}

func samplecompute(ctx context.Context, analytics *sql.DB) error {
	var (
		err   error
		loads []float64
	)

	if loads, err = cpu.PercentWithContext(ctx, 0, false); err != nil {
		return errorsx.Wrap(err, "unable to retrieve compute")
	}

	load := slicesx.FirstOrZero(loads...)

	if err := analytics.QueryRowContext(ctx, "INSERT INTO 'eg.metrics.cpu' (id, name, ts, load) VALUES (?, 'compute', ?, ?) RETURNING load", uuid.Must(uuid.NewV7()).String(), time.Now().UTC(), load).Scan(&load); err != nil {
		return err
	}

	debugx.Println("eg.metrics.compute", load)
	return nil
}

func systemcpu(ctx context.Context, analytics *sql.DB) error {
	if _, err := analytics.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS 'eg.metrics.cpu' (id UUID PRIMARY KEY, name TEXT NOT NULL, ts TIMESTAMP NOT NULL, load FLOAT4 NOT NULL)"); err != nil {
		return err
	}

	return timex.NowAndEvery(ctx, 5*time.Second, func(ctx context.Context) error {
		return samplecompute(ctx, analytics)
	})
}

func samplememory(ctx context.Context, analytics *sql.DB) error {
	const query = "INSERT INTO 'eg.metrics.memory' (id, name, ts, percent, unused, used, cached, total) VALUES (?, 'memory', ?, ?, ?, ?, ?, ?) RETURNING percent"
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

	debugx.Println("eg.metrics.memory.percent", percent)
	return nil
}

func systemmemory(ctx context.Context, analytics *sql.DB) error {
	if _, err := analytics.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS 'eg.metrics.memory' (id UUID PRIMARY KEY, name TEXT NOT NULL, ts TIMESTAMP NOT NULL, percent FLOAT4 NOT NULL, unused INT8 NOT NULL, used INT8 NOT NULL, cached INT8 NOT NULL, total INT8 NOT NULL)"); err != nil {
		return err
	}

	return timex.NowAndEvery(ctx, 5*time.Second, func(ctx context.Context) error {
		return samplememory(ctx, analytics)
	})
}

func systemnet(ctx context.Context, analytics *sql.DB) error {
	if _, err := analytics.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS 'eg.metrics.network' (id UUID PRIMARY KEY, name TEXT NOT NULL, ts TIMESTAMP NOT NULL, bytes_sent UINT64 NOT NULL, bytes_recv UINT64 NOT NULL, packets_sent UINT64 NOT NULL, packets_recv UINT64 NOT NULL, packets_sent_dropped UINT64 NOT NULL, packets_recv_dropped UINT64 NOT NULL, errors_sent UINT64 NOT NULL, errors_recv UINT64 NOT NULL, errors_fifo_sent UINT64 NOT NULL, errors_fifo_recv UINT64 NOT NULL)"); err != nil {
		return err
	}

	return timex.NowAndEvery(ctx, 5*time.Second, func(ctx context.Context) error {
		return samplenet(ctx, analytics)
	})
}

func samplenet(ctx context.Context, analytics *sql.DB) error {
	const query = "INSERT INTO 'eg.metrics.network' (id, name, ts, bytes_sent, bytes_recv, packets_sent, packets_recv, packets_sent_dropped, packets_recv_dropped, errors_sent, errors_recv, errors_fifo_sent, errors_fifo_recv) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING bytes_sent, bytes_recv, packets_sent, packets_recv, packets_sent_dropped, packets_recv_dropped, errors_sent, errors_recv, errors_fifo_sent, errors_fifo_recv"
	var (
		err   error
		usage []psnet.IOCountersStat
	)

	if usage, err = psnet.IOCountersWithContext(ctx, false); err != nil {
		return errorsx.Wrap(err, "unable to retrieve memory usage")
	}

	for _, v := range usage {
		if err := analytics.QueryRowContext(ctx, query, uuid.Must(uuid.NewV7()).String(), v.Name, time.Now().UTC(), v.BytesSent, v.BytesRecv, v.PacketsSent, v.PacketsRecv, v.Dropout, v.Dropin, v.Errout, v.Errin, v.Fifoout, v.Fifoin).Scan(&v.BytesSent, &v.BytesRecv, &v.PacketsSent, &v.PacketsRecv, &v.Dropout, &v.Dropin, &v.Errout, &v.Errin, &v.Fifoout, &v.Fifoin); err != nil {
			return err
		}
		debugx.Printf("eg.metrics.network %s v(sent,recv) bytes(%d, %d) packets(%d,%d) packets_dropped(%d,%d) total_errors(%d,%d) fifo_buff_errors(%d,%d)\n", v.Name, v.BytesSent, v.BytesRecv, v.PacketsSent, v.PacketsRecv, v.Dropout, v.Dropin, v.Errout, v.Errin, v.Fifoout, v.Fifoin)
	}

	return nil
}
