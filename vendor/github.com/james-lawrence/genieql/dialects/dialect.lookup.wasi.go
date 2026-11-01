//go:build wasm

package dialects

import "github.com/james-lawrence/genieql"

// LookupDialect lookup a registered dialect.
func LookupDialect(config genieql.Configuration) (genieql.Dialect, error) {
	var (
		err     error
		factory DialectFactory
	)

	if factory, err = dialects.LookupDialect("wasi"); err != nil {
		return nil, err
	}

	return factory.Connect(config)
}
