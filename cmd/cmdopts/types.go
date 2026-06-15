package cmdopts

import (
	"fmt"
	"math"
	"net"
	"reflect"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/internal/errorsx"
)

// ParseIP addresses
func ParseIP(ctx *kong.DecodeContext, target reflect.Value) (err error) {
	target.Set(reflect.ValueOf(net.ParseIP(ctx.Scan.Pop().String())))
	return nil
}

func ParseTCPAddr(ctx *kong.DecodeContext, target reflect.Value) (err error) {
	if ctx.Scan.Len() == 0 {
		return nil
	}

	var (
		saddr = ctx.Scan.Pop().String()
	)

	var (
		addr *net.TCPAddr
	)

	if addr, err = net.ResolveTCPAddr("tcp", saddr); err != nil {
		return errorsx.Wrapf(err, "unable to resolve tcp address %s - %s", saddr, ctx.Value.Name)
	}

	target.Set(reflect.ValueOf(addr))

	return nil
}

// ParseDurationInf parses a time.Duration flag value. In addition to normal
// time.ParseDuration syntax (e.g. "1h30m"), it accepts "infinity"
// (case-insensitive) as an alias for the maximum representable duration,
// used to signal "no timeout".
func ParseDurationInf(ctx *kong.DecodeContext, target reflect.Value) (err error) {
	t, err := ctx.Scan.PopValue("duration")
	if err != nil {
		return err
	}

	var d time.Duration
	switch v := t.Value.(type) {
	case string:
		switch {
		case strings.EqualFold(v, "infinity"):
			d = time.Duration(math.MaxInt)
		default:
			if d, err = time.ParseDuration(v); err != nil {
				return errorsx.Wrapf(err, "expected duration but got %q", v)
			}
		}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		d = reflect.ValueOf(v).Convert(reflect.TypeOf(time.Duration(0))).Interface().(time.Duration)
	default:
		return fmt.Errorf("expected duration but got %q", v)
	}

	target.Set(reflect.ValueOf(d))
	return nil
}

func ParseTCPAddrArray(ctx *kong.DecodeContext, target reflect.Value) (err error) {

	if ctx.Scan.Len() == 0 {
		return nil
	}

	var (
		results []*net.TCPAddr
		token   = ctx.Scan.Pop().String()
	)

	token = strings.ReplaceAll(token, "\n", " ")
	token = strings.ReplaceAll(token, ",", " ")
	for _, saddr := range strings.Split(token, " ") {
		var (
			addr *net.TCPAddr
		)

		if addr, err = net.ResolveTCPAddr("tcp", saddr); err != nil {
			return errorsx.Wrapf(err, "unable to resolve tcp address %s : %s", saddr, token)
		}

		results = append(results, addr)
	}

	target.Set(reflect.ValueOf(results))
	return nil
}
