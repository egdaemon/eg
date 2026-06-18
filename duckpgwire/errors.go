package duckpgwire

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgproto3"
)

// SQLSTATE codes. duckdb-go has no structured error codes of its own, so
// most errors map to the generic "internal_error" class; only conditions
// duckpgwire itself detects (not duckdb) get a sharper code.
const (
	sqlStateInternalError       = "XX000"
	sqlStateQueryCanceled       = "57014"
	sqlStateIdleInTransaction   = "25P03"
	sqlStateInFailedTransaction = "25P02"
)

func errorResponse(code, message string) *pgproto3.ErrorResponse {
	return &pgproto3.ErrorResponse{
		Severity: "ERROR",
		Code:     code,
		Message:  message,
	}
}

func fatalResponse(code, message string) *pgproto3.ErrorResponse {
	return &pgproto3.ErrorResponse{
		Severity: "FATAL",
		Code:     code,
		Message:  message,
	}
}

// toErrorResponse classifies err into a Postgres ErrorResponse. It special
// cases context cancellation (query_canceled); everything else from
// duckdb-go is a plain Go error string, surfaced under the generic
// internal_error SQLSTATE.
func toErrorResponse(err error) *pgproto3.ErrorResponse {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return errorResponse(sqlStateQueryCanceled, "canceling statement due to timeout or cancel request")
	}
	return errorResponse(sqlStateInternalError, err.Error())
}

var errInFailedTransaction = errors.New("current transaction is aborted, commands ignored until end of transaction block")
