package egnotif

import (
	"context"
	"io"
	"log/slog"
)

// LogOption configures the log deliverer.
type LogOption func(*logOption)
type logOptions []LogOption

type logOption struct {
	level slog.Level
	attrs []any
}

// LogOptions generates the default options for the log deliverer.
func LogOptions() logOptions {
	return logOptions(nil)
}

// Level sets the level completed runs are logged at, failures are always logged at error.
func (t logOptions) Level(l slog.Level) logOptions {
	return append(t, func(o *logOption) {
		o.level = l
	})
}

// Attrs adds attributes (see slog.Logger.Log) to every record.
func (t logOptions) Attrs(attrs ...any) logOptions {
	return append(t, func(o *logOption) {
		o.attrs = append(o.attrs, attrs...)
	})
}

// Log delivers notifications to w using slog.
func Log(w io.Writer, options ...LogOption) deliverer {
	o := logOption{level: slog.LevelInfo}
	for _, opt := range options {
		opt(&o)
	}

	return logger{log: slog.New(slog.NewTextHandler(w, nil)), opts: o}
}

type logger struct {
	log  *slog.Logger
	opts logOption
}

func (t logger) Send(ctx context.Context, v Context) error {
	msg, err := v.Message()
	if err != nil {
		return err
	}

	level := t.opts.level
	if v.Failed() {
		level = slog.LevelError
	}

	attrs := append([]any{
		slog.String("run_id", v.RunID),
		slog.String("account_id", v.AccountID),
		slog.String("vcs", v.VCSURI),
		slog.String("commit", v.Commit),
		slog.String("workload", v.Workload),
		slog.Duration("elapsed", v.Elapsed),
	}, t.opts.attrs...)

	t.log.Log(ctx, level, msg, attrs...)
	return nil
}
