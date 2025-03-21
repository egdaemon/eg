package duckdb_go_bindings

/*
#cgo CPPFLAGS: -DDUCKDB_STATIC_BUILD
#cgo LDFLAGS: -lduckdb -lc++ -L${SRCDIR}/
#include <duckdb.h>
*/
import "C"
