package p2pwebrtc

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	pionlogging "github.com/pion/logging"
)

var logger atomic.Pointer[slog.Logger]

func init() {
	logger.Store(slog.New(slog.DiscardHandler))
}

// SetLogger sets the logger for the package, logging is discarded by default.
// pion's own logging is reported at debug level, it logs routine connection events as errors.
func SetLogger(l *slog.Logger) {
	if l == nil {
		l = slog.New(slog.DiscardHandler)
	}
	logger.Store(l)
}

// log forwards to the current logger so SetLogger applies to code copied from go-libp2p.
var log = proxylogger{}

type proxylogger struct{}

func (proxylogger) Debug(msg string, args ...any) { logger.Load().Debug(msg, args...) }
func (proxylogger) Info(msg string, args ...any)  { logger.Load().Info(msg, args...) }
func (proxylogger) Warn(msg string, args ...any)  { logger.Load().Warn(msg, args...) }
func (proxylogger) Error(msg string, args ...any) { logger.Load().Error(msg, args...) }

type pionfactory struct{}

func (pionfactory) NewLogger(scope string) pionlogging.LeveledLogger {
	return pionlogger{scope: scope}
}

type pionlogger struct {
	scope string
}

func (t pionlogger) emit(msg string) {
	l := logger.Load()
	if !l.Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	l.Debug(msg, "pion", t.scope)
}

func (t pionlogger) Trace(msg string)                  {}
func (t pionlogger) Tracef(format string, args ...any) {}
func (t pionlogger) Debug(msg string)                  { t.emit(msg) }
func (t pionlogger) Debugf(format string, args ...any) { t.emit(fmt.Sprintf(format, args...)) }
func (t pionlogger) Info(msg string)                   { t.emit(msg) }
func (t pionlogger) Infof(format string, args ...any)  { t.emit(fmt.Sprintf(format, args...)) }
func (t pionlogger) Warn(msg string)                   { t.emit(msg) }
func (t pionlogger) Warnf(format string, args ...any)  { t.emit(fmt.Sprintf(format, args...)) }
func (t pionlogger) Error(msg string)                  { t.emit(msg) }
func (t pionlogger) Errorf(format string, args ...any) { t.emit(fmt.Sprintf(format, args...)) }
