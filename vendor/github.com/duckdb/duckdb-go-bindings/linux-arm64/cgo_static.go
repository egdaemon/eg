package duckdb_go_bindings

/*
#cgo CPPFLAGS: -DDUCKDB_STATIC_BUILD
#cgo LDFLAGS: -lcore -lcorefunctions -lparquet -licu -lstdc++ -lm -ldl -L${SRCDIR}/
#include <duckdb.h>
*/
import "C"
