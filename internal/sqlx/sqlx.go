// Package sqlx contains small helpers for working with database/sql that
// arent provided by the standard library.
package sqlx

// database/sql returns this as an unexported sentinel (errDBClosed), so it
// cannot be matched with errors.Is; compare by message instead.
const errClosedMsg = "sql: database is closed"

// IgnoreClosed returns nil when err is database/sql's "database is closed"
// error, and err otherwise. Useful for background workers racing DB shutdown.
func IgnoreClosed(err error) error {
	if err != nil && err.Error() == errClosedMsg {
		return nil
	}

	return err
}
