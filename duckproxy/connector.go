package duckproxy

import (
	"context"
	"database/sql/driver"
	"net"
)

// DialFunc dials the duckproxy socket at addr -- e.g. wasinet.DialContext
// from inside the wasm guest, where plain net.Dial isn't usable.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// NewConnector builds a driver.Connector that dials dsn with dial instead
// of Driver's hardcoded net.Dial, for use with sql.OpenDB.
func NewConnector(dsn string, dial DialFunc) driver.Connector {
	return &connector{dsn: dsn, dial: dial}
}

type connector struct {
	dsn  string
	dial DialFunc
}

func (c *connector) Connect(ctx context.Context) (driver.Conn, error) {
	nc, err := c.dial(ctx, "unix", c.dsn)
	if err != nil {
		return nil, err
	}
	return &conn{c: nc}, nil
}

func (c *connector) Driver() driver.Driver { return &Driver{} }
