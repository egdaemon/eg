package daemons

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/numericx"
	"github.com/egdaemon/eg/internal/userx"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/shirou/gopsutil/v4/disk"
)

type DiskUsage struct {
	Threshold float64       `name:"threshold" help:"percent threshold to trigger services at" default:"80"`
	Period    time.Duration `name:"frequency" help:"frequency to check" default:"1m"`
	Services  []string      `arg:"" name:"services" placeholder:"eg.service" required:"true"`
}

func (t DiskUsage) perform(ctx context.Context, conn *dbus.Conn) error {
	resultToError := func(result string) error {
		if result == "done" {
			return nil
		}
		return errors.New(result)
	}

	startJob := func(ctx context.Context, target string, d func(context.Context, string, string, chan<- string) (int, error)) error {
		await := make(chan string)

		_, err := d(ctx, target, "replace", await)
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-await:
			return resultToError(result)
		}
	}

	parts, _ := disk.Partitions(false)
	max := 0.0
	for _, p := range parts {
		device := p.Mountpoint
		s, err := disk.Usage(device)
		if err != nil {
			log.Println("unable to retrieve usage", p.Mountpoint, err)
			continue
		}

		if s.Total == 0 {
			continue
		}

		log.Println(s.Path, s.UsedPercent)
		max = numericx.Max(max, s.UsedPercent)
	}

	if max < t.Threshold {
		debugx.Println("usage below threshold", max, "<", t.Threshold)
		return nil
	} else {
		log.Println("clearing disk space due to threshhold", max, ">", t.Threshold)
	}

	for _, s := range t.Services {
		if err := errorsx.Wrap(startJob(ctx, s, conn.RestartUnitContext), "systemd restart unit failed"); err != nil {
			return err
		}
	}

	return nil
}
func (t DiskUsage) Run(gctx *cmdopts.Global) (err error) {
	var (
		conn *dbus.Conn
	)

	if u := userx.CurrentUserOrDefault(userx.Root()); u.Uid == "0" {
		if conn, err = dbus.NewSystemConnectionContext(gctx.Context); err != nil {
			return errorsx.Wrap(err, "failed to connect to systemd bus")
		}
	} else {
		if conn, err = dbus.NewSystemConnectionContext(gctx.Context); err != nil {
			return errorsx.Wrap(err, "failed to connect to systemd bus")
		}
	}

	log.Println("services that will be started", t.Services)

	if err = t.perform(gctx.Context, conn); err != nil {
		return err
	}

	for {
		select {
		case <-gctx.Context.Done():
			return gctx.Context.Err()
		case <-time.After(t.Period):
			if err = t.perform(gctx.Context, conn); err != nil {
				return err
			}
		}
	}
}
