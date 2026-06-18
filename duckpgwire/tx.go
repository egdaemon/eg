package duckpgwire

import (
	"regexp"
	"strings"
)

// txState tracks transaction status for a session. DuckDB's own
// transaction-open flag isn't exported by the driver, so duckpgwire tracks
// it independently by sniffing BEGIN/COMMIT/ROLLBACK keywords in executed
// SQL text, mirroring Postgres's ReadyForQuery status byte semantics.
type txState int

const (
	txIdle txState = iota
	txInTransaction
	txFailed
)

// statusByte returns the byte Postgres's ReadyForQuery message reports for
// this state: 'I' idle, 'T' in a transaction block, 'E' in a failed
// transaction block.
func (t txState) statusByte() byte {
	switch t {
	case txInTransaction:
		return 'T'
	case txFailed:
		return 'E'
	default:
		return 'I'
	}
}

type txKeyword int

const (
	txKeywordNone txKeyword = iota
	txKeywordBegin
	txKeywordCommit
	txKeywordRollback
)

var (
	reBegin    = regexp.MustCompile(`(?i)^\s*(BEGIN(\s+TRANSACTION)?|START\s+TRANSACTION)\b`)
	reCommit   = regexp.MustCompile(`(?i)^\s*(COMMIT|END)\b`)
	reRollback = regexp.MustCompile(`(?i)^\s*ROLLBACK\b`)
)

// classifyTxKeyword reports which transaction-control statement sql is, if
// any. Only the first statement of a (possibly multi-statement) string is
// considered.
func classifyTxKeyword(sql string) txKeyword {
	sql = strings.TrimSpace(sql)

	switch {
	case reBegin.MatchString(sql):
		return txKeywordBegin
	case reCommit.MatchString(sql):
		return txKeywordCommit
	case reRollback.MatchString(sql):
		return txKeywordRollback
	default:
		return txKeywordNone
	}
}
