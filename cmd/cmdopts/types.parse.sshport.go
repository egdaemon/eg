package cmdopts

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/alecthomas/kong"
)

// DefaultSSHPort is the port published for the SSH server when a bare --ssh
// flag is provided without an explicit port override.
const DefaultSSHPort = 2222

// SSHPortMapper implements a bool-like Kong flag: bare --ssh enables the
// feature with DefaultSSHPort, while --ssh=<port> enables it with a specific
// port. Omitting the flag entirely leaves the target at its zero value,
// which callers treat as "disabled".
type SSHPortMapper struct{}

func (SSHPortMapper) Decode(ctx *kong.DecodeContext, target reflect.Value) error {
	if ctx.Scan.Peek().Type != kong.FlagValueToken {
		target.SetInt(DefaultSSHPort)
		return nil
	}

	t, err := ctx.Scan.PopValue("int")
	if err != nil {
		return err
	}

	switch v := t.Value.(type) {
	case string:
		port, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("expected port but got %q", v)
		}
		target.SetInt(int64(port))
	case int, int8, int16, int32, int64:
		target.SetInt(reflect.ValueOf(v).Convert(reflect.TypeFor[int64]()).Int())
	default:
		return fmt.Errorf("expected port but got %q (%T)", t.Value, t.Value)
	}

	return nil
}

func (SSHPortMapper) IsBool() bool { return true }
