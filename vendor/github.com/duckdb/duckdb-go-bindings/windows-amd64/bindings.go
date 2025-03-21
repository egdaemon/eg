package duckdb_go_bindings

/*
#include <duckdb.h>
*/
import "C"

import (
	"log"
	"sync/atomic"
	"unsafe"
)

// ------------------------------------------------------------------ //
// Enums
// ------------------------------------------------------------------ //

// Type wraps duckdb_type.
type Type = C.duckdb_type

const (
	TypeInvalid     Type = C.DUCKDB_TYPE_INVALID
	TypeBoolean     Type = C.DUCKDB_TYPE_BOOLEAN
	TypeTinyInt     Type = C.DUCKDB_TYPE_TINYINT
	TypeSmallInt    Type = C.DUCKDB_TYPE_SMALLINT
	TypeInteger     Type = C.DUCKDB_TYPE_INTEGER
	TypeBigInt      Type = C.DUCKDB_TYPE_BIGINT
	TypeUTinyInt    Type = C.DUCKDB_TYPE_UTINYINT
	TypeUSmallInt   Type = C.DUCKDB_TYPE_USMALLINT
	TypeUInteger    Type = C.DUCKDB_TYPE_UINTEGER
	TypeUBigInt     Type = C.DUCKDB_TYPE_UBIGINT
	TypeFloat       Type = C.DUCKDB_TYPE_FLOAT
	TypeDouble      Type = C.DUCKDB_TYPE_DOUBLE
	TypeTimestamp   Type = C.DUCKDB_TYPE_TIMESTAMP
	TypeDate        Type = C.DUCKDB_TYPE_DATE
	TypeTime        Type = C.DUCKDB_TYPE_TIME
	TypeInterval    Type = C.DUCKDB_TYPE_INTERVAL
	TypeHugeInt     Type = C.DUCKDB_TYPE_HUGEINT
	TypeUHugeInt    Type = C.DUCKDB_TYPE_UHUGEINT
	TypeVarchar     Type = C.DUCKDB_TYPE_VARCHAR
	TypeBlob        Type = C.DUCKDB_TYPE_BLOB
	TypeDecimal     Type = C.DUCKDB_TYPE_DECIMAL
	TypeTimestampS  Type = C.DUCKDB_TYPE_TIMESTAMP_S
	TypeTimestampMS Type = C.DUCKDB_TYPE_TIMESTAMP_MS
	TypeTimestampNS Type = C.DUCKDB_TYPE_TIMESTAMP_NS
	TypeEnum        Type = C.DUCKDB_TYPE_ENUM
	TypeList        Type = C.DUCKDB_TYPE_LIST
	TypeStruct      Type = C.DUCKDB_TYPE_STRUCT
	TypeMap         Type = C.DUCKDB_TYPE_MAP
	TypeArray       Type = C.DUCKDB_TYPE_ARRAY
	TypeUUID        Type = C.DUCKDB_TYPE_UUID
	TypeUnion       Type = C.DUCKDB_TYPE_UNION
	TypeBit         Type = C.DUCKDB_TYPE_BIT
	TypeTimeTZ      Type = C.DUCKDB_TYPE_TIME_TZ
	TypeTimestampTZ Type = C.DUCKDB_TYPE_TIMESTAMP_TZ
	TypeAny         Type = C.DUCKDB_TYPE_ANY
	TypeVarInt      Type = C.DUCKDB_TYPE_VARINT
	TypeSQLNull     Type = C.DUCKDB_TYPE_SQLNULL
)

// State wraps duckdb_state.
type State = C.duckdb_state

const (
	StateSuccess State = C.DuckDBSuccess
	StateError   State = C.DuckDBError
)

// PendingState wraps duckdb_pending_state.
type PendingState = C.duckdb_pending_state

const (
	PendingStateResultReady      PendingState = C.DUCKDB_PENDING_RESULT_READY
	PendingStateResultNotReady   PendingState = C.DUCKDB_PENDING_RESULT_NOT_READY
	PendingStateError            PendingState = C.DUCKDB_PENDING_ERROR
	PendingStateNoTasksAvailable PendingState = C.DUCKDB_PENDING_NO_TASKS_AVAILABLE
)

// ResultType wraps duckdb_result_type.
type ResultType = C.duckdb_result_type

const (
	ResultTypeInvalid     ResultType = C.DUCKDB_RESULT_TYPE_INVALID
	ResultTypeChangedRows ResultType = C.DUCKDB_RESULT_TYPE_CHANGED_ROWS
	ResultTypeNothing     ResultType = C.DUCKDB_RESULT_TYPE_NOTHING
	ResultTypeQueryResult ResultType = C.DUCKDB_RESULT_TYPE_QUERY_RESULT
)

// StatementType wraps duckdb_statement_type.
type StatementType = C.duckdb_statement_type

const (
	StatementTypeInvalid     StatementType = C.DUCKDB_STATEMENT_TYPE_INVALID
	StatementTypeSelect      StatementType = C.DUCKDB_STATEMENT_TYPE_SELECT
	StatementTypeInsert      StatementType = C.DUCKDB_STATEMENT_TYPE_INSERT
	StatementTypeUpdate      StatementType = C.DUCKDB_STATEMENT_TYPE_UPDATE
	StatementTypeExplain     StatementType = C.DUCKDB_STATEMENT_TYPE_EXPLAIN
	StatementTypeDelete      StatementType = C.DUCKDB_STATEMENT_TYPE_DELETE
	StatementTypePrepare     StatementType = C.DUCKDB_STATEMENT_TYPE_PREPARE
	StatementTypeCreate      StatementType = C.DUCKDB_STATEMENT_TYPE_CREATE
	StatementTypeExecute     StatementType = C.DUCKDB_STATEMENT_TYPE_EXECUTE
	StatementTypeAlter       StatementType = C.DUCKDB_STATEMENT_TYPE_ALTER
	StatementTypeTransaction StatementType = C.DUCKDB_STATEMENT_TYPE_TRANSACTION
	StatementTypeCopy        StatementType = C.DUCKDB_STATEMENT_TYPE_COPY
	StatementTypeAnalyze     StatementType = C.DUCKDB_STATEMENT_TYPE_ANALYZE
	StatementTypeVariableSet StatementType = C.DUCKDB_STATEMENT_TYPE_VARIABLE_SET
	StatementTypeCreateFunc  StatementType = C.DUCKDB_STATEMENT_TYPE_CREATE_FUNC
	StatementTypeDrop        StatementType = C.DUCKDB_STATEMENT_TYPE_DROP
	StatementTypeExport      StatementType = C.DUCKDB_STATEMENT_TYPE_EXPORT
	StatementTypePragma      StatementType = C.DUCKDB_STATEMENT_TYPE_PRAGMA
	StatementTypeVacuum      StatementType = C.DUCKDB_STATEMENT_TYPE_VACUUM
	StatementTypeCall        StatementType = C.DUCKDB_STATEMENT_TYPE_CALL
	StatementTypeSet         StatementType = C.DUCKDB_STATEMENT_TYPE_SET
	StatementTypeLoad        StatementType = C.DUCKDB_STATEMENT_TYPE_LOAD
	StatementTypeRelation    StatementType = C.DUCKDB_STATEMENT_TYPE_RELATION
	StatementTypeExtension   StatementType = C.DUCKDB_STATEMENT_TYPE_EXTENSION
	StatementTypeLogicalPlan StatementType = C.DUCKDB_STATEMENT_TYPE_LOGICAL_PLAN
	StatementTypeAttach      StatementType = C.DUCKDB_STATEMENT_TYPE_ATTACH
	StatementTypeDetach      StatementType = C.DUCKDB_STATEMENT_TYPE_DETACH
	StatementTypeMulti       StatementType = C.DUCKDB_STATEMENT_TYPE_MULTI
)

// ErrorType wraps duckdb_error_type.
type ErrorType = C.duckdb_error_type

const (
	ErrorTypeInvalid              ErrorType = C.DUCKDB_ERROR_INVALID
	ErrorTypeOutOfRange           ErrorType = C.DUCKDB_ERROR_OUT_OF_RANGE
	ErrorTypeConversion           ErrorType = C.DUCKDB_ERROR_CONVERSION
	ErrorTypeUnknownType          ErrorType = C.DUCKDB_ERROR_UNKNOWN_TYPE
	ErrorTypeDecimal              ErrorType = C.DUCKDB_ERROR_DECIMAL
	ErrorTypeMismatchType         ErrorType = C.DUCKDB_ERROR_MISMATCH_TYPE
	ErrorTypeDivideByZero         ErrorType = C.DUCKDB_ERROR_DIVIDE_BY_ZERO
	ErrorTypeObjectSize           ErrorType = C.DUCKDB_ERROR_OBJECT_SIZE
	ErrorTypeInvalidType          ErrorType = C.DUCKDB_ERROR_INVALID_TYPE
	ErrorTypeSerialization        ErrorType = C.DUCKDB_ERROR_SERIALIZATION
	ErrorTypeTransaction          ErrorType = C.DUCKDB_ERROR_TRANSACTION
	ErrorTypeNotImplemented       ErrorType = C.DUCKDB_ERROR_NOT_IMPLEMENTED
	ErrorTypeExpression           ErrorType = C.DUCKDB_ERROR_EXPRESSION
	ErrorTypeCatalog              ErrorType = C.DUCKDB_ERROR_CATALOG
	ErrorTypeParser               ErrorType = C.DUCKDB_ERROR_PARSER
	ErrorTypePlanner              ErrorType = C.DUCKDB_ERROR_PLANNER
	ErrorTypeScheduler            ErrorType = C.DUCKDB_ERROR_SCHEDULER
	ErrorTypeExecutor             ErrorType = C.DUCKDB_ERROR_EXECUTOR
	ErrorTypeConstraint           ErrorType = C.DUCKDB_ERROR_CONSTRAINT
	ErrorTypeIndex                ErrorType = C.DUCKDB_ERROR_INDEX
	ErrorTypeStat                 ErrorType = C.DUCKDB_ERROR_STAT
	ErrorTypeConnection           ErrorType = C.DUCKDB_ERROR_CONNECTION
	ErrorTypeSyntax               ErrorType = C.DUCKDB_ERROR_SYNTAX
	ErrorTypeSettings             ErrorType = C.DUCKDB_ERROR_SETTINGS
	ErrorTypeBinder               ErrorType = C.DUCKDB_ERROR_BINDER
	ErrorTypeNetwork              ErrorType = C.DUCKDB_ERROR_NETWORK
	ErrorTypeOptimizer            ErrorType = C.DUCKDB_ERROR_OPTIMIZER
	ErrorTypeNullPointer          ErrorType = C.DUCKDB_ERROR_NULL_POINTER
	ErrorTypeErrorIO              ErrorType = C.DUCKDB_ERROR_IO
	ErrorTypeInterrupt            ErrorType = C.DUCKDB_ERROR_INTERRUPT
	ErrorTypeFatal                ErrorType = C.DUCKDB_ERROR_FATAL
	ErrorTypeInternal             ErrorType = C.DUCKDB_ERROR_INTERNAL
	ErrorTypeInvalidInput         ErrorType = C.DUCKDB_ERROR_INVALID_INPUT
	ErrorTypeOutOfMemory          ErrorType = C.DUCKDB_ERROR_OUT_OF_MEMORY
	ErrorTypePermission           ErrorType = C.DUCKDB_ERROR_PERMISSION
	ErrorTypeParameterNotResolved ErrorType = C.DUCKDB_ERROR_PARAMETER_NOT_RESOLVED
	ErrorTypeParameterNotAllowed  ErrorType = C.DUCKDB_ERROR_PARAMETER_NOT_ALLOWED
	ErrorTypeDependency           ErrorType = C.DUCKDB_ERROR_DEPENDENCY
	ErrorTypeHTTP                 ErrorType = C.DUCKDB_ERROR_HTTP
	ErrorTypeMissingExtension     ErrorType = C.DUCKDB_ERROR_MISSING_EXTENSION
	ErrorTypeAutoload             ErrorType = C.DUCKDB_ERROR_AUTOLOAD
	ErrorTypeSequence             ErrorType = C.DUCKDB_ERROR_SEQUENCE
	ErrorTypeInvalidConfiguration ErrorType = C.DUCKDB_INVALID_CONFIGURATION
)

// CastMode wraps duckdb_cast_mode.
type CastMode = C.duckdb_cast_mode

const (
	CastModeNormal CastMode = C.DUCKDB_CAST_NORMAL
	CastModeTry    CastMode = C.DUCKDB_CAST_TRY
)

// ------------------------------------------------------------------ //
// Types
// ------------------------------------------------------------------ //

type IdxT = C.idx_t

// Types without internal pointers:

type (
	Date              = C.duckdb_date
	DateStruct        = C.duckdb_date_struct
	Time              = C.duckdb_time
	TimeStruct        = C.duckdb_time_struct
	TimeTZ            = C.duckdb_time_tz
	TimeTZStruct      = C.duckdb_time_tz_struct
	Timestamp         = C.duckdb_timestamp
	TimestampStruct   = C.duckdb_timestamp_struct
	Interval          = C.duckdb_interval
	HugeInt           = C.duckdb_hugeint
	UHugeInt          = C.duckdb_uhugeint
	Decimal           = C.duckdb_decimal
	QueryProgressType = C.duckdb_query_progress_type
	StringT           = C.duckdb_string_t
	ListEntry         = C.duckdb_list_entry
)

// duckdb_string
// duckdb_blob
// duckdb_extension_access

// Helper functions for types without internal pointers:

// NewDate sets the members of a duckdb_date.
func NewDate(days int32) *Date {
	return &Date{days: C.int32_t(days)}
}

// DateStructMembers returns the year, month, and day of a duckdb_date.
func DateStructMembers(date *DateStruct) (int32, int8, int8) {
	return int32(date.year), int8(date.month), int8(date.day)
}

// NewTime sets the members of a duckdb_time.
func NewTime(micros int64) *Time {
	return &Time{micros: C.int64_t(micros)}
}

// TimeMembers returns the micros of a duckdb_time.
func TimeMembers(ti *Time) int64 {
	return int64(ti.micros)
}

// TimeStructMembers returns the hour, min, sec, and micros of a duckdb_time_struct.
func TimeStructMembers(ti *TimeStruct) (int8, int8, int8, int32) {
	return int8(ti.hour), int8(ti.min), int8(ti.sec), int32(ti.micros)
}

// TimeTZStructMembers returns the time and offset of a duckdb_time_tz_struct.
func TimeTZStructMembers(ti *TimeTZStruct) (TimeStruct, int32) {
	return ti.time, int32(ti.offset)
}

// NewTimestamp sets the members of a duckdb_timestamp.
func NewTimestamp(micros int64) *Timestamp {
	return &Timestamp{micros: C.int64_t(micros)}
}

// TimestampMembers returns the micros of a duckdb_timestamp.
func TimestampMembers(ts *Timestamp) int64 {
	return int64(ts.micros)
}

// NewInterval sets the members of a duckdb_interval.
func NewInterval(months int32, days int32, micros int64) *Interval {
	return &Interval{
		months: C.int32_t(months),
		days:   C.int32_t(days),
		micros: C.int64_t(micros),
	}
}

// IntervalMembers returns the months, days, and micros of a duckdb_interval.
func IntervalMembers(i *Interval) (int32, int32, int64) {
	return int32(i.months), int32(i.days), int64(i.micros)
}

// NewHugeInt sets the members of a duckdb_hugeint.
func NewHugeInt(lower uint64, upper int64) *HugeInt {
	return &HugeInt{
		lower: C.uint64_t(lower),
		upper: C.int64_t(upper),
	}
}

// HugeIntMembers returns the lower and upper of a duckdb_hugeint.
func HugeIntMembers(hi *HugeInt) (uint64, int64) {
	return uint64(hi.lower), int64(hi.upper)
}

// NewListEntry sets the members of a duckdb_list_entry.
func NewListEntry(offset uint64, length uint64) *ListEntry {
	return &ListEntry{
		offset: C.uint64_t(offset),
		length: C.uint64_t(length),
	}
}

// ListEntryMembers returns the offset and length of a duckdb_list_entry.
func ListEntryMembers(entry *ListEntry) (uint64, uint64) {
	return uint64(entry.offset), uint64(entry.length)
}

// Types with internal pointers:

// duckdb_column

// Result wraps duckdb_result.
// NOTE: Using 'type Result = C.duckdb_result' causes a somewhat mysterious
// 'runtime error: cgo argument has Go pointer to unpinned Go pointer'.
// See https://github.com/golang/go/issues/28606#issuecomment-2184269962.
// When using a type alias, duckdb_result itself contains a Go unsafe.Pointer for its 'void *internal_data' field.
type Result struct {
	data C.duckdb_result
}

// ------------------------------------------------------------------ //
// Pointer Types
// ------------------------------------------------------------------ //

// NOTE: No wrappings for function pointers.
// *duckdb_delete_callback_t
// *duckdb_task_state
// *duckdb_scalar_function_t
// *duckdb_aggregate_state_size
// *duckdb_aggregate_init_t
// *duckdb_aggregate_destroy_t
// *duckdb_aggregate_update_t
// *duckdb_aggregate_combine_t
// *duckdb_aggregate_finalize_t
// *duckdb_table_function_bind_t
// *duckdb_table_function_init_t
// *duckdb_table_function_t
// *duckdb_cast_function_t
// *duckdb_replacement_callback_t

// NOTE: We export the Ptr of each wrapped type pointer to allow (void *) typedef's of callback functions.
// See https://golang.org/issue/19837 and https://golang.org/issue/19835.

// NOTE: For some types (e.g., Appender, but not Config) omitting the Ptr causes
// the same somewhat mysterious runtime error as described for Result.
// 'runtime error: cgo argument has Go pointer to unpinned Go pointer'.
// See https://github.com/golang/go/issues/28606#issuecomment-2184269962.
// When using a type alias, duckdb_result itself contains a Go unsafe.Pointer for its 'void *internal_ptr' field.

// *duckdb_task_state

// Vector wraps *duckdb_vector.
type Vector struct {
	Ptr unsafe.Pointer
}

func (vec *Vector) data() C.duckdb_vector {
	return C.duckdb_vector(vec.Ptr)
}

// Database wraps *duckdb_database.
type Database struct {
	Ptr unsafe.Pointer
}

func (db *Database) data() C.duckdb_database {
	return C.duckdb_database(db.Ptr)
}

// Connection wraps *duckdb_connection.
type Connection struct {
	Ptr unsafe.Pointer
}

func (conn *Connection) data() C.duckdb_connection {
	return C.duckdb_connection(conn.Ptr)
}

// PreparedStatement wraps *duckdb_prepared_statement.
type PreparedStatement struct {
	Ptr unsafe.Pointer
}

func (preparedStmt *PreparedStatement) data() C.duckdb_prepared_statement {
	return C.duckdb_prepared_statement(preparedStmt.Ptr)
}

// ExtractedStatements wraps *duckdb_extracted_statements.
type ExtractedStatements struct {
	Ptr unsafe.Pointer
}

func (extractedStmts *ExtractedStatements) data() C.duckdb_extracted_statements {
	return C.duckdb_extracted_statements(extractedStmts.Ptr)
}

// PendingResult wraps *duckdb_pending_result.
type PendingResult struct {
	Ptr unsafe.Pointer
}

func (pendingRes *PendingResult) data() C.duckdb_pending_result {
	return C.duckdb_pending_result(pendingRes.Ptr)
}

// Appender wraps *duckdb_appender.
type Appender struct {
	Ptr unsafe.Pointer
}

func (appender *Appender) data() C.duckdb_appender {
	return C.duckdb_appender(appender.Ptr)
}

// TableDescription wraps *duckdb_table_description.
type TableDescription struct {
	Ptr unsafe.Pointer
}

func (description *TableDescription) data() C.duckdb_table_description {
	return C.duckdb_table_description(description.Ptr)
}

// Config wraps *duckdb_config.
type Config struct {
	Ptr unsafe.Pointer
}

func (config *Config) data() C.duckdb_config {
	return C.duckdb_config(config.Ptr)
}

// LogicalType wraps *duckdb_logical_type.
type LogicalType struct {
	Ptr unsafe.Pointer
}

func (logicalType *LogicalType) data() C.duckdb_logical_type {
	return C.duckdb_logical_type(logicalType.Ptr)
}

// *duckdb_create_type_info

// DataChunk wraps *duckdb_data_chunk.
type DataChunk struct {
	Ptr unsafe.Pointer
}

func (chunk *DataChunk) data() C.duckdb_data_chunk {
	return C.duckdb_data_chunk(chunk.Ptr)
}

// Value wraps *duckdb_value.
type Value struct {
	Ptr unsafe.Pointer
}

func (v *Value) data() C.duckdb_value {
	return C.duckdb_value(v.Ptr)
}

// ProfilingInfo wraps *duckdb_profiling_info.
type ProfilingInfo struct {
	Ptr unsafe.Pointer
}

func (info *ProfilingInfo) data() C.duckdb_profiling_info {
	return C.duckdb_profiling_info(info.Ptr)
}

// *duckdb_extension_info

// FunctionInfo wraps *duckdb_function_info.
type FunctionInfo struct {
	Ptr unsafe.Pointer
}

func (info *FunctionInfo) data() C.duckdb_function_info {
	return C.duckdb_function_info(info.Ptr)
}

// ScalarFunction wraps *duckdb_scalar_function.
type ScalarFunction struct {
	Ptr unsafe.Pointer
}

func (f *ScalarFunction) data() C.duckdb_scalar_function {
	return C.duckdb_scalar_function(f.Ptr)
}

// ScalarFunctionSet wraps *duckdb_scalar_function_set.
type ScalarFunctionSet struct {
	Ptr unsafe.Pointer
}

func (set *ScalarFunctionSet) data() C.duckdb_scalar_function_set {
	return C.duckdb_scalar_function_set(set.Ptr)
}

// *duckdb_aggregate_function
// *duckdb_aggregate_function_set
// *duckdb_aggregate_state

// TableFunction wraps *duckdb_table_function.
type TableFunction struct {
	Ptr unsafe.Pointer
}

func (f *TableFunction) data() C.duckdb_table_function {
	return C.duckdb_table_function(f.Ptr)
}

// BindInfo wraps *duckdb_bind_info.
type BindInfo struct {
	Ptr unsafe.Pointer
}

func (info *BindInfo) data() C.duckdb_bind_info {
	return C.duckdb_bind_info(info.Ptr)
}

// InitInfo wraps *C.duckdb_init_info.
type InitInfo struct {
	Ptr unsafe.Pointer
}

func (info *InitInfo) data() C.duckdb_init_info {
	return C.duckdb_init_info(info.Ptr)
}

// *duckdb_cast_function

// ReplacementScanInfo wraps *duckdb_replacement_scan.
type ReplacementScanInfo struct {
	Ptr unsafe.Pointer
}

func (info *ReplacementScanInfo) data() C.duckdb_replacement_scan_info {
	return C.duckdb_replacement_scan_info(info.Ptr)
}

// Arrow wraps *duckdb_arrow.
type Arrow struct {
	Ptr unsafe.Pointer
}

func (arrow *Arrow) data() C.duckdb_arrow {
	return C.duckdb_arrow(arrow.Ptr)
}

// ArrowStream wraps *duckdb_arrow_stream.
type ArrowStream struct {
	Ptr unsafe.Pointer
}

func (stream *ArrowStream) data() C.duckdb_arrow_stream {
	return C.duckdb_arrow_stream(stream.Ptr)
}

// ArrowSchema wraps *duckdb_arrow_schema.
type ArrowSchema struct {
	Ptr unsafe.Pointer
}

func (schema *ArrowSchema) data() C.duckdb_arrow_schema {
	return C.duckdb_arrow_schema(schema.Ptr)
}

// ArrowArray wraps *duckdb_arrow_array.
type ArrowArray struct {
	Ptr unsafe.Pointer
}

func (array *ArrowArray) data() C.duckdb_arrow_array {
	return C.duckdb_arrow_array(array.Ptr)
}

// ------------------------------------------------------------------ //
// Functions
// ------------------------------------------------------------------ //

// ------------------------------------------------------------------ //
// Open Connect
// ------------------------------------------------------------------ //

// duckdb_open

// OpenExt wraps duckdb_open_ext.
// outDb must be closed with Close.
func OpenExt(path string, outDb *Database, config Config, errMsg *string) State {
	cPath := C.CString(path)
	defer Free(unsafe.Pointer(cPath))
	var err *C.char
	defer Free(unsafe.Pointer(err))

	var db C.duckdb_database
	state := C.duckdb_open_ext(cPath, &db, config.data(), &err)
	outDb.Ptr = unsafe.Pointer(db)
	*errMsg = C.GoString(err)

	if debugMode {
		allocCounters.db.Add(1)
	}
	return state
}

// Close wraps duckdb_close.
func Close(db *Database) {
	if debugMode {
		allocCounters.db.Add(-1)
	}
	if db.Ptr == nil {
		return
	}
	data := db.data()
	C.duckdb_close(&data)
	db.Ptr = nil
}

// Connect wraps duckdb_connect.
// outConn must be disconnected with Disconnect.
func Connect(db Database, outConn *Connection) State {
	var conn C.duckdb_connection
	state := C.duckdb_connect(db.data(), &conn)
	outConn.Ptr = unsafe.Pointer(conn)
	if debugMode {
		allocCounters.conn.Add(1)
	}
	return state
}

func Interrupt(conn Connection) {
	C.duckdb_interrupt(conn.data())
}

// duckdb_query_progress

// Disconnect wraps duckdb_disconnect.
func Disconnect(conn *Connection) {
	if debugMode {
		allocCounters.conn.Add(-1)
	}
	if conn.Ptr == nil {
		return
	}
	data := conn.data()
	C.duckdb_disconnect(&data)
	conn.Ptr = nil
}

// duckdb_library_version

// ------------------------------------------------------------------ //
// Configuration
// ------------------------------------------------------------------ //

// CreateConfig wraps duckdb_create_config.
// outConfig must be destroyed with DestroyConfig.
func CreateConfig(outConfig *Config) State {
	var config C.duckdb_config
	state := C.duckdb_create_config(&config)
	outConfig.Ptr = unsafe.Pointer(config)
	if debugMode {
		allocCounters.config.Add(1)
	}
	return state
}

// duckdb_config_count
// duckdb_get_config_flag

func SetConfig(config Config, name string, option string) State {
	cName := C.CString(name)
	defer Free(unsafe.Pointer(cName))
	cOption := C.CString(option)
	defer Free(unsafe.Pointer(cOption))
	return C.duckdb_set_config(config.data(), cName, cOption)
}

// DestroyConfig wraps duckdb_destroy_config.
func DestroyConfig(config *Config) {
	if debugMode {
		allocCounters.config.Add(-1)
	}
	if config.Ptr == nil {
		return
	}
	data := config.data()
	C.duckdb_destroy_config(&data)
	config.Ptr = nil
}

// ------------------------------------------------------------------ //
// Query Execution
// ------------------------------------------------------------------ //

// duckdb_query

// DestroyResult wraps duckdb_destroy_result.
func DestroyResult(res *Result) {
	if debugMode {
		allocCounters.res.Add(-1)
	}
	if res == nil {
		return
	}
	C.duckdb_destroy_result(&res.data)
	res = nil
}

func ColumnName(res *Result, col IdxT) string {
	name := C.duckdb_column_name(&res.data, col)
	return C.GoString(name)
}

func ColumnType(res *Result, col IdxT) Type {
	return C.duckdb_column_type(&res.data, col)
}

// duckdb_result_statement_type

// ColumnLogicalType wraps duckdb_column_logical_type.
// The return value must be destroyed with DestroyLogicalType.
func ColumnLogicalType(res *Result, col IdxT) LogicalType {
	logicalType := C.duckdb_column_logical_type(&res.data, col)
	if debugMode {
		allocCounters.logicalType.Add(1)
	}
	return LogicalType{
		Ptr: unsafe.Pointer(logicalType),
	}
}

func ColumnCount(res *Result) IdxT {
	return C.duckdb_column_count(&res.data)
}

// duckdb_rows_changed

func ResultError(res *Result) string {
	err := C.duckdb_result_error(&res.data)
	return C.GoString(err)
}

// duckdb_result_error_type

// ------------------------------------------------------------------ //
// Result Functions (many are deprecated)
// ------------------------------------------------------------------ //

// duckdb_result_return_type

// ------------------------------------------------------------------ //
// Safe Fetch Functions (all deprecated)
// ------------------------------------------------------------------ //

// ------------------------------------------------------------------ //
// Helpers
// ------------------------------------------------------------------ //

// duckdb_malloc

func Free(ptr unsafe.Pointer) {
	C.duckdb_free(ptr)
}

func VectorSize() IdxT {
	return C.duckdb_vector_size()
}

func StringIsInlined(strT StringT) bool {
	isInlined := C.duckdb_string_is_inlined(strT)
	return bool(isInlined)
}

func StringTLength(strT StringT) uint32 {
	length := C.duckdb_string_t_length(strT)
	return uint32(length)
}

func StringTData(strT *StringT) string {
	length := C.int(StringTLength(*strT))
	ptr := unsafe.Pointer(C.duckdb_string_t_data(strT))
	return string(C.GoBytes(ptr, length))
}

// ------------------------------------------------------------------ //
// Date Time Timestamp Helpers
// ------------------------------------------------------------------ //

func FromDate(date Date) DateStruct {
	return C.duckdb_from_date(date)
}

// duckdb_to_date
// duckdb_is_finite_date
// duckdb_from_time

func CreateTimeTZ(micros int64, offset int32) TimeTZ {
	return C.duckdb_create_time_tz(C.int64_t(micros), C.int32_t(offset))
}

func FromTimeTZ(ti TimeTZ) TimeTZStruct {
	return C.duckdb_from_time_tz(ti)
}

// duckdb_to_time
// duckdb_from_timestamp
// duckdb_to_timestamp
// duckdb_is_finite_timestamp
// duckdb_is_finite_timestamp_s
// duckdb_is_finite_timestamp_ms
// duckdb_is_finite_timestamp_ns

// ------------------------------------------------------------------ //
// Hugeint Helpers
// ------------------------------------------------------------------ //

// duckdb_hugeint_to_double
// duckdb_double_to_hugeint

// ------------------------------------------------------------------ //
// Unsigned Hugeint Helpers
// ------------------------------------------------------------------ //

// duckdb_uhugeint_to_double
// duckdb_double_to_uhugeint

// ------------------------------------------------------------------ //
// Decimal Helpers
// ------------------------------------------------------------------ //

// duckdb_double_to_decimal
// duckdb_decimal_to_double

// ------------------------------------------------------------------ //
// Prepared Statements
// ------------------------------------------------------------------ //

// duckdb_prepare

// DestroyPrepare wraps duckdb_destroy_prepare.
func DestroyPrepare(preparedStmt *PreparedStatement) {
	if debugMode {
		allocCounters.preparedStmt.Add(-1)
	}
	if preparedStmt.Ptr == nil {
		return
	}
	data := preparedStmt.data()
	C.duckdb_destroy_prepare(&data)
	preparedStmt.Ptr = nil
}

func PrepareError(preparedStmt PreparedStatement) string {
	err := C.duckdb_prepare_error(preparedStmt.data())
	return C.GoString(err)
}

func NParams(preparedStmt PreparedStatement) IdxT {
	return C.duckdb_nparams(preparedStmt.data())
}

func ParameterName(preparedStmt PreparedStatement, index IdxT) string {
	cName := C.duckdb_parameter_name(preparedStmt.data(), index)
	defer Free(unsafe.Pointer(cName))
	return C.GoString(cName)
}

func ParamType(preparedStmt PreparedStatement, index IdxT) Type {
	return C.duckdb_param_type(preparedStmt.data(), index)
}

// duckdb_param_logical_type
// duckdb_clear_bindings

func PreparedStatementType(preparedStmt PreparedStatement) StatementType {
	return C.duckdb_prepared_statement_type(preparedStmt.data())
}

// ------------------------------------------------------------------ //
// Bind Values To Prepared Statements
// ------------------------------------------------------------------ //

func BindValue(preparedStmt PreparedStatement, index IdxT, v Value) State {
	return C.duckdb_bind_value(preparedStmt.data(), index, v.data())
}

// duckdb_bind_parameter_index

func BindBoolean(preparedStmt PreparedStatement, index IdxT, v bool) State {
	return C.duckdb_bind_boolean(preparedStmt.data(), index, C.bool(v))
}

func BindInt8(preparedStmt PreparedStatement, index IdxT, v int8) State {
	return C.duckdb_bind_int8(preparedStmt.data(), index, C.int8_t(v))
}

func BindInt16(preparedStmt PreparedStatement, index IdxT, v int16) State {
	return C.duckdb_bind_int16(preparedStmt.data(), index, C.int16_t(v))
}

func BindInt32(preparedStmt PreparedStatement, index IdxT, v int32) State {
	return C.duckdb_bind_int32(preparedStmt.data(), index, C.int32_t(v))
}

func BindInt64(preparedStmt PreparedStatement, index IdxT, v int64) State {
	return C.duckdb_bind_int64(preparedStmt.data(), index, C.int64_t(v))
}

func BindHugeInt(preparedStmt PreparedStatement, index IdxT, v HugeInt) State {
	return C.duckdb_bind_hugeint(preparedStmt.data(), index, v)
}

// duckdb_bind_uhugeint

func BindDecimal(preparedStmt PreparedStatement, index IdxT, v Decimal) State {
	return C.duckdb_bind_decimal(preparedStmt.data(), index, v)
}

func BindUInt8(preparedStmt PreparedStatement, index IdxT, v uint8) State {
	return C.duckdb_bind_uint8(preparedStmt.data(), index, C.uint8_t(v))
}

func BindUInt16(preparedStmt PreparedStatement, index IdxT, v uint16) State {
	return C.duckdb_bind_uint16(preparedStmt.data(), index, C.uint16_t(v))
}

func BindUInt32(preparedStmt PreparedStatement, index IdxT, v uint32) State {
	return C.duckdb_bind_uint32(preparedStmt.data(), index, C.uint32_t(v))
}

func BindUInt64(preparedStmt PreparedStatement, index IdxT, v uint64) State {
	return C.duckdb_bind_uint64(preparedStmt.data(), index, C.uint64_t(v))
}

func BindFloat(preparedStmt PreparedStatement, index IdxT, v float32) State {
	return C.duckdb_bind_float(preparedStmt.data(), index, C.float(v))
}

func BindDouble(preparedStmt PreparedStatement, index IdxT, v float64) State {
	return C.duckdb_bind_double(preparedStmt.data(), index, C.double(v))
}

func BindDate(preparedStmt PreparedStatement, index IdxT, v Date) State {
	return C.duckdb_bind_date(preparedStmt.data(), index, v)
}

func BindTime(preparedStmt PreparedStatement, index IdxT, v Time) State {
	return C.duckdb_bind_time(preparedStmt.data(), index, v)
}

func BindTimestamp(preparedStmt PreparedStatement, index IdxT, v Timestamp) State {
	return C.duckdb_bind_timestamp(preparedStmt.data(), index, v)
}

// duckdb_bind_timestamp_tz

func BindInterval(preparedStmt PreparedStatement, index IdxT, v Interval) State {
	return C.duckdb_bind_interval(preparedStmt.data(), index, v)
}

func BindVarchar(preparedStmt PreparedStatement, index IdxT, v string) State {
	cStr := C.CString(v)
	defer Free(unsafe.Pointer(cStr))
	return C.duckdb_bind_varchar(preparedStmt.data(), index, cStr)
}

// duckdb_bind_varchar_length

func BindBlob(preparedStmt PreparedStatement, index IdxT, v []byte) State {
	cBytes := C.CBytes(v)
	defer Free(unsafe.Pointer(cBytes))
	return C.duckdb_bind_blob(preparedStmt.data(), index, cBytes, IdxT(len(v)))
}

func BindNull(preparedStmt PreparedStatement, index IdxT) State {
	return C.duckdb_bind_null(preparedStmt.data(), index)
}

// ------------------------------------------------------------------ //
// Execute Prepared Statements (many are deprecated)
// ------------------------------------------------------------------ //

// duckdb_execute_prepared

// ------------------------------------------------------------------ //
// Extract Statements
// ------------------------------------------------------------------ //

// ExtractStatements wraps duckdb_extract_statements.
// outExtractedStmts must be destroyed with DestroyExtracted.
func ExtractStatements(conn Connection, query string, outExtractedStmts *ExtractedStatements) IdxT {
	cQuery := C.CString(query)
	defer Free(unsafe.Pointer(cQuery))

	var extractedStmts C.duckdb_extracted_statements
	count := C.duckdb_extract_statements(conn.data(), cQuery, &extractedStmts)
	outExtractedStmts.Ptr = unsafe.Pointer(extractedStmts)
	if debugMode {
		allocCounters.extractedStmts.Add(1)
	}
	return count
}

// PrepareExtractedStatement wraps duckdb_prepare_extracted_statement.
// outPreparedStmt must be destroyed with DestroyPrepare.
func PrepareExtractedStatement(conn Connection, extractedStmts ExtractedStatements, index IdxT, outPreparedStmt *PreparedStatement) State {
	var preparedStmt C.duckdb_prepared_statement
	state := C.duckdb_prepare_extracted_statement(conn.data(), extractedStmts.data(), index, &preparedStmt)
	outPreparedStmt.Ptr = unsafe.Pointer(preparedStmt)
	if debugMode {
		allocCounters.preparedStmt.Add(1)
	}
	return state
}

func ExtractStatementsError(extractedStmts ExtractedStatements) string {
	err := C.duckdb_extract_statements_error(extractedStmts.data())
	return C.GoString(err)
}

// DestroyExtracted wraps duckdb_destroy_extracted.
func DestroyExtracted(extractedStmts *ExtractedStatements) {
	if debugMode {
		allocCounters.extractedStmts.Add(-1)
	}
	if extractedStmts.Ptr == nil {
		return
	}
	data := extractedStmts.data()
	C.duckdb_destroy_extracted(&data)
	extractedStmts.Ptr = nil
}

// ------------------------------------------------------------------ //
// Pending Result Interface
// ------------------------------------------------------------------ //

// PendingPrepared wraps duckdb_pending_prepared.
// outPendingRes must be destroyed with DestroyPending.
func PendingPrepared(preparedStmt PreparedStatement, outPendingRes *PendingResult) State {
	var pendingRes C.duckdb_pending_result
	state := C.duckdb_pending_prepared(preparedStmt.data(), &pendingRes)
	outPendingRes.Ptr = unsafe.Pointer(pendingRes)
	if debugMode {
		allocCounters.pendingRes.Add(1)
	}
	return state
}

// DestroyPending wraps duckdb_destroy_pending.
func DestroyPending(pendingRes *PendingResult) {
	if debugMode {
		allocCounters.pendingRes.Add(-1)
	}
	if pendingRes.Ptr == nil {
		return
	}
	data := pendingRes.data()
	C.duckdb_destroy_pending(&data)
	pendingRes.Ptr = nil
}

func PendingError(pendingRes PendingResult) string {
	err := C.duckdb_pending_error(pendingRes.data())
	return C.GoString(err)
}

// duckdb_pending_execute_task
// duckdb_pending_execute_check_state

// ExecutePending wraps duckdb_execute_pending.
// outRes must be destroyed with DestroyResult.
func ExecutePending(res PendingResult, outRes *Result) State {
	if debugMode {
		allocCounters.res.Add(1)
	}
	return C.duckdb_execute_pending(res.data(), &outRes.data)
}

// duckdb_pending_execution_is_finished

// ------------------------------------------------------------------ //
// Value Interface
// ------------------------------------------------------------------ //

// DestroyValue wraps duckdb_destroy_value.
func DestroyValue(v *Value) {
	if debugMode {
		allocCounters.v.Add(-1)
	}
	if v.Ptr == nil {
		return
	}
	data := v.data()
	C.duckdb_destroy_value(&data)
	v.Ptr = nil
}

// CreateVarchar wraps duckdb_create_varchar.
// The return value must be destroyed with DestroyValue.
func CreateVarchar(str string) Value {
	cStr := C.CString(str)
	defer Free(unsafe.Pointer(cStr))
	v := C.duckdb_create_varchar(cStr)
	if debugMode {
		allocCounters.v.Add(1)
	}
	return Value{
		Ptr: unsafe.Pointer(v),
	}
}

// duckdb_create_varchar_length
// duckdb_create_bool
// duckdb_create_int8
// duckdb_create_uint8
// duckdb_create_int16
// duckdb_create_uint16
// duckdb_create_int32
// duckdb_create_uint32
// duckdb_create_uint64

// CreateInt64 wraps duckdb_create_int64.
// The return value must be destroyed with DestroyValue.
func CreateInt64(val int64) Value {
	v := C.duckdb_create_int64(C.int64_t(val))
	if debugMode {
		allocCounters.v.Add(1)
	}
	return Value{
		Ptr: unsafe.Pointer(v),
	}
}

// duckdb_create_hugeint
// duckdb_create_uhugeint
// duckdb_create_varint
// duckdb_create_decimal
// duckdb_create_float
// duckdb_create_double
// duckdb_create_date
// duckdb_create_time

// CreateTimeTZValue wraps duckdb_create_time_tz_value.
// The return value must be destroyed with DestroyValue.
func CreateTimeTZValue(timeTZ TimeTZ) Value {
	v := C.duckdb_create_time_tz_value(timeTZ)
	if debugMode {
		allocCounters.v.Add(1)
	}
	return Value{
		Ptr: unsafe.Pointer(v),
	}
}

// duckdb_create_timestamp
// duckdb_create_timestamp_tz
// duckdb_create_timestamp_s
// duckdb_create_timestamp_ms
// duckdb_create_timestamp_ns
// duckdb_create_interval
// duckdb_create_blob
// duckdb_create_bit
// duckdb_create_uuid

func GetBool(v Value) bool {
	val := C.duckdb_get_bool(v.data())
	return bool(val)
}

func GetInt8(v Value) int8 {
	val := C.duckdb_get_int8(v.data())
	return int8(val)
}

func GetUInt8(v Value) uint8 {
	val := C.duckdb_get_uint8(v.data())
	return uint8(val)
}

func GetInt16(v Value) int16 {
	val := C.duckdb_get_int16(v.data())
	return int16(val)
}

func GetUInt16(v Value) uint16 {
	val := C.duckdb_get_uint16(v.data())
	return uint16(val)
}

func GetInt32(v Value) int32 {
	val := C.duckdb_get_int32(v.data())
	return int32(val)
}

func GetUInt32(v Value) uint32 {
	val := C.duckdb_get_uint32(v.data())
	return uint32(val)
}

func GetInt64(v Value) int64 {
	val := C.duckdb_get_int64(v.data())
	return int64(val)
}

func GetUInt64(v Value) uint64 {
	val := C.duckdb_get_uint64(v.data())
	return uint64(val)
}

func GetHugeInt(v Value) HugeInt {
	return C.duckdb_get_hugeint(v.data())
}

// duckdb_get_uhugeint
// duckdb_get_varint
// duckdb_get_decimal

func GetFloat(v Value) float32 {
	val := C.duckdb_get_float(v.data())
	return float32(val)
}

func GetDouble(v Value) float64 {
	val := C.duckdb_get_double(v.data())
	return float64(val)
}

func GetDate(v Value) Date {
	return C.duckdb_get_date(v.data())
}

func GetTime(v Value) Time {
	return C.duckdb_get_time(v.data())
}

func GetTimeTZ(v Value) TimeTZ {
	return C.duckdb_get_time_tz(v.data())
}

func GetTimestamp(v Value) Timestamp {
	return C.duckdb_get_timestamp(v.data())
}

// duckdb_get_timestamp_tz
// duckdb_get_timestamp_s
// duckdb_get_timestamp_ms
// duckdb_get_timestamp_ns

func GetInterval(v Value) Interval {
	return C.duckdb_get_interval(v.data())
}

// duckdb_get_value_type
// duckdb_get_blob
// duckdb_get_bit
// duckdb_get_uuid

func GetVarchar(v Value) string {
	cStr := C.duckdb_get_varchar(v.data())
	defer Free(unsafe.Pointer(cStr))
	return C.GoString(cStr)
}

// duckdb_create_struct_value
// duckdb_create_list_value
// duckdb_create_array_value

func GetMapSize(v Value) IdxT {
	return C.duckdb_get_map_size(v.data())
}

// GetMapKey wraps duckdb_get_map_key.
// The return value must be destroyed with DestroyValue.
func GetMapKey(v Value, index IdxT) Value {
	value := C.duckdb_get_map_key(v.data(), index)
	if debugMode {
		allocCounters.v.Add(1)
	}
	return Value{
		Ptr: unsafe.Pointer(value),
	}
}

// GetMapValue wraps duckdb_get_map_value.
// The return value must be destroyed with DestroyValue.
func GetMapValue(v Value, index IdxT) Value {
	value := C.duckdb_get_map_value(v.data(), index)
	if debugMode {
		allocCounters.v.Add(1)
	}
	return Value{
		Ptr: unsafe.Pointer(value),
	}
}

// duckdb_is_null_value
// duckdb_create_null_value
// duckdb_get_list_size
// duckdb_get_list_child
// duckdb_create_enum_value
// duckdb_get_enum_value
// duckdb_get_struct_child

// ------------------------------------------------------------------ //
// Logical Type Interface
// ------------------------------------------------------------------ //

// CreateLogicalType wraps duckdb_create_logical_type.
// The return value must be destroyed with DestroyLogicalType.
func CreateLogicalType(t Type) LogicalType {
	logicalType := C.duckdb_create_logical_type(t)
	if debugMode {
		allocCounters.logicalType.Add(1)
	}
	return LogicalType{
		Ptr: unsafe.Pointer(logicalType),
	}
}

func LogicalTypeGetAlias(logicalType LogicalType) string {
	alias := C.duckdb_logical_type_get_alias(logicalType.data())
	defer Free(unsafe.Pointer(alias))
	return C.GoString(alias)
}

// duckdb_logical_type_set_alias

// CreateListType wraps duckdb_create_list_type.
// The return value must be destroyed with DestroyLogicalType.
func CreateListType(child LogicalType) LogicalType {
	logicalType := C.duckdb_create_list_type(child.data())
	if debugMode {
		allocCounters.logicalType.Add(1)
	}
	return LogicalType{
		Ptr: unsafe.Pointer(logicalType),
	}
}

// CreateArrayType wraps duckdb_create_array_type.
// The return value must be destroyed with DestroyLogicalType.
func CreateArrayType(child LogicalType, size IdxT) LogicalType {
	logicalType := C.duckdb_create_array_type(child.data(), size)
	if debugMode {
		allocCounters.logicalType.Add(1)
	}
	return LogicalType{
		Ptr: unsafe.Pointer(logicalType),
	}
}

// CreateMapType wraps duckdb_create_map_type.
// The return value must be destroyed with DestroyLogicalType.
func CreateMapType(key LogicalType, value LogicalType) LogicalType {
	logicalType := C.duckdb_create_map_type(key.data(), value.data())
	if debugMode {
		allocCounters.logicalType.Add(1)
	}
	return LogicalType{
		Ptr: unsafe.Pointer(logicalType),
	}
}

// duckdb_create_union_type

// CreateStructType wraps duckdb_create_struct_type.
// The return value must be destroyed with DestroyLogicalType.
func CreateStructType(types []LogicalType, names []string) LogicalType {
	count := len(types)

	typesSlice := allocLogicalTypeSlice(types)
	defer Free(typesSlice)
	typesPtr := (*C.duckdb_logical_type)(typesSlice)

	namesSlice := (*[1 << 31]*C.char)(C.malloc(C.size_t(count) * charSize))
	defer Free(unsafe.Pointer(namesSlice))
	for i, name := range names {
		(*namesSlice)[i] = C.CString(name)
	}
	namesPtr := (**C.char)(unsafe.Pointer(namesSlice))

	// Create the STRUCT type.
	logicalType := C.duckdb_create_struct_type(typesPtr, namesPtr, IdxT(count))
	for i := 0; i < count; i++ {
		Free(unsafe.Pointer((*namesSlice)[i]))
	}

	if debugMode {
		allocCounters.logicalType.Add(1)
	}
	return LogicalType{
		Ptr: unsafe.Pointer(logicalType),
	}
}

// CreateEnumType wraps duckdb_create_enum_type.
// The return value must be destroyed with DestroyLogicalType.
func CreateEnumType(names []string) LogicalType {
	count := len(names)

	namesSlice := (*[1 << 31]*C.char)(C.malloc(C.size_t(count) * charSize))
	defer Free(unsafe.Pointer(namesSlice))
	for i, name := range names {
		(*namesSlice)[i] = C.CString(name)
	}
	namesPtr := (**C.char)(unsafe.Pointer(namesSlice))

	// Create the ENUM type.
	logicalType := C.duckdb_create_enum_type(namesPtr, IdxT(count))
	for i := 0; i < count; i++ {
		Free(unsafe.Pointer((*namesSlice)[i]))
	}

	if debugMode {
		allocCounters.logicalType.Add(1)
	}
	return LogicalType{
		Ptr: unsafe.Pointer(logicalType),
	}
}

// CreateDecimalType wraps duckdb_create_decimal_type.
// The return value must be destroyed with DestroyLogicalType.
func CreateDecimalType(width uint8, scale uint8) LogicalType {
	logicalType := C.duckdb_create_decimal_type(C.uint8_t(width), C.uint8_t(scale))
	if debugMode {
		allocCounters.logicalType.Add(1)
	}
	return LogicalType{
		Ptr: unsafe.Pointer(logicalType),
	}
}

func GetTypeId(logicalType LogicalType) Type {
	return C.duckdb_get_type_id(logicalType.data())
}

func DecimalWidth(logicalType LogicalType) uint8 {
	width := C.duckdb_decimal_width(logicalType.data())
	return uint8(width)
}

func DecimalScale(logicalType LogicalType) uint8 {
	scale := C.duckdb_decimal_scale(logicalType.data())
	return uint8(scale)
}

func DecimalInternalType(logicalType LogicalType) Type {
	return C.duckdb_decimal_internal_type(logicalType.data())
}

func EnumInternalType(logicalType LogicalType) Type {
	return C.duckdb_enum_internal_type(logicalType.data())
}

func EnumDictionarySize(logicalType LogicalType) uint32 {
	size := C.duckdb_enum_dictionary_size(logicalType.data())
	return uint32(size)
}

func EnumDictionaryValue(logicalType LogicalType, index IdxT) string {
	str := C.duckdb_enum_dictionary_value(logicalType.data(), index)
	defer Free(unsafe.Pointer(str))
	return C.GoString(str)
}

// ListTypeChildType wraps duckdb_list_type_child_type.
// The return value must be destroyed with DestroyLogicalType.
func ListTypeChildType(logicalType LogicalType) LogicalType {
	child := C.duckdb_list_type_child_type(logicalType.data())
	if debugMode {
		allocCounters.logicalType.Add(1)
	}
	return LogicalType{
		Ptr: unsafe.Pointer(child),
	}
}

// ArrayTypeChildType wraps duckdb_array_type_child_type.
// The return value must be destroyed with DestroyLogicalType.
func ArrayTypeChildType(logicalType LogicalType) LogicalType {
	child := C.duckdb_array_type_child_type(logicalType.data())
	if debugMode {
		allocCounters.logicalType.Add(1)
	}
	return LogicalType{
		Ptr: unsafe.Pointer(child),
	}
}

func ArrayTypeArraySize(logicalType LogicalType) IdxT {
	return C.duckdb_array_type_array_size(logicalType.data())
}

// MapTypeKeyType wraps duckdb_map_type_key_type.
// The return value must be destroyed with DestroyLogicalType.
func MapTypeKeyType(logicalType LogicalType) LogicalType {
	key := C.duckdb_map_type_key_type(logicalType.data())
	if debugMode {
		allocCounters.logicalType.Add(1)
	}
	return LogicalType{
		Ptr: unsafe.Pointer(key),
	}
}

// MapTypeValueType wraps duckdb_map_type_value_type.
// The return value must be destroyed with DestroyLogicalType.
func MapTypeValueType(logicalType LogicalType) LogicalType {
	value := C.duckdb_map_type_value_type(logicalType.data())
	if debugMode {
		allocCounters.logicalType.Add(1)
	}
	return LogicalType{
		Ptr: unsafe.Pointer(value),
	}
}

func StructTypeChildCount(logicalType LogicalType) IdxT {
	return C.duckdb_struct_type_child_count(logicalType.data())
}

func StructTypeChildName(logicalType LogicalType, index IdxT) string {
	cName := C.duckdb_struct_type_child_name(logicalType.data(), index)
	defer Free(unsafe.Pointer(cName))
	return C.GoString(cName)
}

// StructTypeChildType wraps duckdb_struct_type_child_type.
// The return value must be destroyed with DestroyLogicalType.
func StructTypeChildType(logicalType LogicalType, index IdxT) LogicalType {
	child := C.duckdb_struct_type_child_type(logicalType.data(), index)
	if debugMode {
		allocCounters.logicalType.Add(1)
	}
	return LogicalType{
		Ptr: unsafe.Pointer(child),
	}
}

// duckdb_union_type_member_count
// duckdb_union_type_member_name
// duckdb_union_type_member_type

// DestroyLogicalType wraps duckdb_destroy_logical_type.
func DestroyLogicalType(logicalType *LogicalType) {
	if debugMode {
		allocCounters.logicalType.Add(-1)
	}
	if logicalType.Ptr == nil {
		return
	}
	data := logicalType.data()
	C.duckdb_destroy_logical_type(&data)
	logicalType.Ptr = nil
}

// duckdb_register_logical_type

// ------------------------------------------------------------------ //
// Data Chunk Interface
// ------------------------------------------------------------------ //

// CreateDataChunk wraps duckdb_create_data_chunk.
// The return value must be destroyed with DestroyDataChunk.
func CreateDataChunk(types []LogicalType) DataChunk {
	count := len(types)

	typesSlice := allocLogicalTypeSlice(types)
	typesPtr := (*C.duckdb_logical_type)(typesSlice)
	defer Free(unsafe.Pointer(typesPtr))

	chunk := C.duckdb_create_data_chunk(typesPtr, IdxT(count))
	if debugMode {
		allocCounters.chunk.Add(1)
	}
	return DataChunk{
		Ptr: unsafe.Pointer(chunk),
	}
}

// DestroyDataChunk wraps duckdb_destroy_data_chunk.
func DestroyDataChunk(chunk *DataChunk) {
	if debugMode {
		allocCounters.chunk.Add(-1)
	}
	if chunk.Ptr == nil {
		return
	}
	data := chunk.data()
	C.duckdb_destroy_data_chunk(&data)
	chunk.Ptr = nil
}

// duckdb_data_chunk_reset

func DataChunkGetColumnCount(chunk DataChunk) IdxT {
	return C.duckdb_data_chunk_get_column_count(chunk.data())
}

func DataChunkGetVector(chunk DataChunk, index IdxT) Vector {
	vec := C.duckdb_data_chunk_get_vector(chunk.data(), index)
	return Vector{
		Ptr: unsafe.Pointer(vec),
	}
}

func DataChunkGetSize(chunk DataChunk) IdxT {
	return C.duckdb_data_chunk_get_size(chunk.data())
}

func DataChunkSetSize(chunk DataChunk, size IdxT) {
	C.duckdb_data_chunk_set_size(chunk.data(), size)
}

// ------------------------------------------------------------------ //
// Vector Interface
// ------------------------------------------------------------------ //

// VectorGetColumnType wraps duckdb_vector_get_column_type.
// The return value must be destroyed with DestroyLogicalType.
func VectorGetColumnType(vec Vector) LogicalType {
	logicalType := C.duckdb_vector_get_column_type(vec.data())
	if debugMode {
		allocCounters.logicalType.Add(1)
	}
	return LogicalType{
		Ptr: unsafe.Pointer(logicalType),
	}
}

func VectorGetData(vec Vector) unsafe.Pointer {
	ptr := C.duckdb_vector_get_data(vec.data())
	return unsafe.Pointer(ptr)
}

func VectorGetValidity(vec Vector) unsafe.Pointer {
	mask := C.duckdb_vector_get_validity(vec.data())
	return unsafe.Pointer(mask)
}

func VectorEnsureValidityWritable(vec Vector) {
	C.duckdb_vector_ensure_validity_writable(vec.data())
}

func VectorAssignStringElement(vec Vector, index IdxT, str string) {
	cStr := C.CString(str)
	defer Free(unsafe.Pointer(cStr))
	C.duckdb_vector_assign_string_element(vec.data(), index, cStr)
}

func VectorAssignStringElementLen(vec Vector, index IdxT, blob []byte, len IdxT) {
	cBytes := (*C.char)(C.CBytes(blob))
	defer Free(unsafe.Pointer(cBytes))
	C.duckdb_vector_assign_string_element_len(vec.data(), index, cBytes, len)
}

func ListVectorGetChild(vec Vector) Vector {
	child := C.duckdb_list_vector_get_child(vec.data())
	return Vector{
		Ptr: unsafe.Pointer(child),
	}
}

func ListVectorGetSize(vec Vector) IdxT {
	return C.duckdb_list_vector_get_size(vec.data())
}

func ListVectorSetSize(vec Vector, size IdxT) State {
	return C.duckdb_list_vector_set_size(vec.data(), size)
}

func ListVectorReserve(vec Vector, capacity IdxT) State {
	return C.duckdb_list_vector_reserve(vec.data(), capacity)
}

func StructVectorGetChild(vec Vector, index IdxT) Vector {
	child := C.duckdb_struct_vector_get_child(vec.data(), index)
	return Vector{
		Ptr: unsafe.Pointer(child),
	}
}

func ArrayVectorGetChild(vec Vector) Vector {
	child := C.duckdb_array_vector_get_child(vec.data())
	return Vector{
		Ptr: unsafe.Pointer(child),
	}
}

// ------------------------------------------------------------------ //
// Validity Mask Functions
// ------------------------------------------------------------------ //

// duckdb_validity_row_is_valid
// duckdb_validity_set_row_validity

func ValiditySetRowInvalid(maskPtr unsafe.Pointer, row IdxT) {
	mask := (*C.uint64_t)(maskPtr)
	C.duckdb_validity_set_row_invalid(mask, row)
}

// duckdb_validity_set_row_valid

// ------------------------------------------------------------------ //
// Scalar Functions
// ------------------------------------------------------------------ //

// CreateScalarFunction wraps duckdb_create_scalar_function.
// The return value must be destroyed with DestroyScalarFunction.
func CreateScalarFunction() ScalarFunction {
	f := C.duckdb_create_scalar_function()
	if debugMode {
		allocCounters.scalarFunc.Add(1)
	}
	return ScalarFunction{
		Ptr: unsafe.Pointer(f),
	}
}

// DestroyScalarFunction wraps duckdb_destroy_scalar_function.
func DestroyScalarFunction(f *ScalarFunction) {
	if debugMode {
		allocCounters.scalarFunc.Add(-1)
	}
	if f.Ptr == nil {
		return
	}
	data := f.data()
	C.duckdb_destroy_scalar_function(&data)
	f.Ptr = nil
}

func ScalarFunctionSetName(f ScalarFunction, name string) {
	cName := C.CString(name)
	defer Free(unsafe.Pointer(cName))
	C.duckdb_scalar_function_set_name(f.data(), cName)
}

func ScalarFunctionSetVarargs(f ScalarFunction, logicalType LogicalType) {
	C.duckdb_scalar_function_set_varargs(f.data(), logicalType.data())
}

func ScalarFunctionSetSpecialHandling(f ScalarFunction) {
	C.duckdb_scalar_function_set_special_handling(f.data())
}

func ScalarFunctionSetVolatile(f ScalarFunction) {
	C.duckdb_scalar_function_set_volatile(f.data())
}

func ScalarFunctionAddParameter(f ScalarFunction, logicalType LogicalType) {
	C.duckdb_scalar_function_add_parameter(f.data(), logicalType.data())
}

func ScalarFunctionSetReturnType(f ScalarFunction, logicalType LogicalType) {
	C.duckdb_scalar_function_set_return_type(f.data(), logicalType.data())
}

func ScalarFunctionSetExtraInfo(f ScalarFunction, extraInfoPtr unsafe.Pointer, callbackPtr unsafe.Pointer) {
	callback := C.duckdb_delete_callback_t(callbackPtr)
	C.duckdb_scalar_function_set_extra_info(f.data(), extraInfoPtr, callback)
}

func ScalarFunctionSetFunction(f ScalarFunction, callbackPtr unsafe.Pointer) {
	callback := C.duckdb_scalar_function_t(callbackPtr)
	C.duckdb_scalar_function_set_function(f.data(), callback)
}

func RegisterScalarFunction(conn Connection, f ScalarFunction) State {
	return C.duckdb_register_scalar_function(conn.data(), f.data())
}

func ScalarFunctionGetExtraInfo(info FunctionInfo) unsafe.Pointer {
	ptr := C.duckdb_scalar_function_get_extra_info(info.data())
	return unsafe.Pointer(ptr)
}

func ScalarFunctionSetError(info FunctionInfo, err string) {
	cErr := C.CString(err)
	defer Free(unsafe.Pointer(cErr))
	C.duckdb_scalar_function_set_error(info.data(), cErr)
}

// CreateScalarFunctionSet wraps duckdb_create_scalar_function_set.
// The return value must be destroyed with DestroyScalarFunctionSet.
func CreateScalarFunctionSet(name string) ScalarFunctionSet {
	cName := C.CString(name)
	defer Free(unsafe.Pointer(cName))

	set := C.duckdb_create_scalar_function_set(cName)
	if debugMode {
		allocCounters.scalarFuncSet.Add(1)
	}
	return ScalarFunctionSet{
		Ptr: unsafe.Pointer(set),
	}
}

// DestroyScalarFunctionSet wraps duckdb_destroy_scalar_function_set.
func DestroyScalarFunctionSet(set *ScalarFunctionSet) {
	if debugMode {
		allocCounters.scalarFuncSet.Add(-1)
	}
	if set.Ptr == nil {
		return
	}
	data := set.data()
	C.duckdb_destroy_scalar_function_set(&data)
	set.Ptr = nil
}

func AddScalarFunctionToSet(set ScalarFunctionSet, f ScalarFunction) State {
	return C.duckdb_add_scalar_function_to_set(set.data(), f.data())
}

func RegisterScalarFunctionSet(conn Connection, f ScalarFunctionSet) State {
	return C.duckdb_register_scalar_function_set(conn.data(), f.data())
}

// ------------------------------------------------------------------ //
// Aggregate Functions
// ------------------------------------------------------------------ //

// duckdb_create_aggregate_function
// duckdb_destroy_aggregate_function
// duckdb_aggregate_function_set_name
// duckdb_aggregate_function_add_parameter
// duckdb_aggregate_function_set_return_type
// duckdb_aggregate_function_set_functions
// duckdb_aggregate_function_set_destructor
// duckdb_register_aggregate_function
// duckdb_aggregate_function_set_special_handling
// duckdb_aggregate_function_set_extra_info
// duckdb_aggregate_function_get_extra_info
// duckdb_aggregate_function_set_error
// duckdb_create_aggregate_function_set
// duckdb_destroy_aggregate_function_set
// duckdb_add_aggregate_function_to_set
// duckdb_register_aggregate_function_set

// ------------------------------------------------------------------ //
// Table Functions
// ------------------------------------------------------------------ //

// CreateTableFunction wraps duckdb_create_table_function.
// The return value must be destroyed with DestroyTableFunction.
func CreateTableFunction() TableFunction {
	f := C.duckdb_create_table_function()
	if debugMode {
		allocCounters.tableFunc.Add(1)
	}
	return TableFunction{
		Ptr: unsafe.Pointer(f),
	}
}

// DestroyTableFunction wraps duckdb_destroy_table_function.
func DestroyTableFunction(f *TableFunction) {
	if debugMode {
		allocCounters.tableFunc.Add(-1)
	}
	if f.Ptr == nil {
		return
	}
	data := f.data()
	C.duckdb_destroy_table_function(&data)
	f.Ptr = nil
}

func TableFunctionSetName(f TableFunction, name string) {
	cName := C.CString(name)
	defer Free(unsafe.Pointer(cName))
	C.duckdb_table_function_set_name(f.data(), cName)
}

func TableFunctionAddParameter(f TableFunction, logicalType LogicalType) {
	C.duckdb_table_function_add_parameter(f.data(), logicalType.data())
}

func TableFunctionAddNamedParameter(f TableFunction, name string, logicalType LogicalType) {
	cName := C.CString(name)
	defer Free(unsafe.Pointer(cName))
	C.duckdb_table_function_add_named_parameter(f.data(), cName, logicalType.data())
}

func TableFunctionSetExtraInfo(f TableFunction, extraInfoPtr unsafe.Pointer, callbackPtr unsafe.Pointer) {
	callback := C.duckdb_delete_callback_t(callbackPtr)
	C.duckdb_table_function_set_extra_info(f.data(), extraInfoPtr, callback)
}

func TableFunctionSetBind(f TableFunction, callbackPtr unsafe.Pointer) {
	callback := C.duckdb_table_function_bind_t(callbackPtr)
	C.duckdb_table_function_set_bind(f.data(), callback)
}

func TableFunctionSetInit(f TableFunction, callbackPtr unsafe.Pointer) {
	callback := C.duckdb_table_function_init_t(callbackPtr)
	C.duckdb_table_function_set_init(f.data(), callback)
}

func TableFunctionSetLocalInit(f TableFunction, callbackPtr unsafe.Pointer) {
	callback := C.duckdb_table_function_init_t(callbackPtr)
	C.duckdb_table_function_set_local_init(f.data(), callback)
}

func TableFunctionSetFunction(f TableFunction, callbackPtr unsafe.Pointer) {
	callback := C.duckdb_table_function_t(callbackPtr)
	C.duckdb_table_function_set_function(f.data(), callback)
}

func TableFunctionSupportsProjectionPushdown(f TableFunction, pushdown bool) {
	C.duckdb_table_function_supports_projection_pushdown(f.data(), C.bool(pushdown))
}

func RegisterTableFunction(conn Connection, f TableFunction) State {
	return C.duckdb_register_table_function(conn.data(), f.data())
}

// ------------------------------------------------------------------ //
// Table Function Bind
// ------------------------------------------------------------------ //

func BindGetExtraInfo(info BindInfo) unsafe.Pointer {
	ptr := C.duckdb_bind_get_extra_info(info.data())
	return unsafe.Pointer(ptr)
}

func BindAddResultColumn(info BindInfo, name string, logicalType LogicalType) {
	cName := C.CString(name)
	defer Free(unsafe.Pointer(cName))
	C.duckdb_bind_add_result_column(info.data(), cName, logicalType.data())
}

// duckdb_bind_get_parameter_count

// BindGetParameter wraps duckdb_bind_get_parameter.
// The return value must be destroyed with DestroyValue.
func BindGetParameter(info BindInfo, index IdxT) Value {
	v := C.duckdb_bind_get_parameter(info.data(), index)
	if debugMode {
		allocCounters.v.Add(1)
	}
	return Value{
		Ptr: unsafe.Pointer(v),
	}
}

// BindGetNamedParameter wraps duckdb_bind_get_named_parameter.
// The return value must be destroyed with DestroyValue.
func BindGetNamedParameter(info BindInfo, name string) Value {
	cName := C.CString(name)
	defer Free(unsafe.Pointer(cName))
	v := C.duckdb_bind_get_named_parameter(info.data(), cName)
	if debugMode {
		allocCounters.v.Add(1)
	}
	return Value{
		Ptr: unsafe.Pointer(v),
	}
}

func BindSetBindData(info BindInfo, bindDataPtr unsafe.Pointer, callbackPtr unsafe.Pointer) {
	callback := C.duckdb_delete_callback_t(callbackPtr)
	C.duckdb_bind_set_bind_data(info.data(), bindDataPtr, callback)
}

func BindSetCardinality(info BindInfo, cardinality IdxT, exact bool) {
	C.duckdb_bind_set_cardinality(info.data(), cardinality, C.bool(exact))
}

func BindSetError(info BindInfo, err string) {
	cErr := C.CString(err)
	defer Free(unsafe.Pointer(cErr))
	C.duckdb_bind_set_error(info.data(), cErr)
}

// ------------------------------------------------------------------ //
// Table Function Init
// ------------------------------------------------------------------ //

// duckdb_init_get_extra_info

func InitGetBindData(info InitInfo) unsafe.Pointer {
	ptr := C.duckdb_init_get_bind_data(info.data())
	return unsafe.Pointer(ptr)
}

func InitSetInitData(info InitInfo, initDataPtr unsafe.Pointer, callbackPtr unsafe.Pointer) {
	callback := C.duckdb_delete_callback_t(callbackPtr)
	C.duckdb_init_set_init_data(info.data(), initDataPtr, callback)
}

func InitGetColumnCount(info InitInfo) IdxT {
	return C.duckdb_init_get_column_count(info.data())
}

func InitGetColumnIndex(info InitInfo, index IdxT) IdxT {
	return C.duckdb_init_get_column_index(info.data(), index)
}

func InitSetMaxThreads(info InitInfo, max IdxT) {
	C.duckdb_init_set_max_threads(info.data(), max)
}

// duckdb_init_set_error

// ------------------------------------------------------------------ //
// Table Function
// ------------------------------------------------------------------ //

// duckdb_function_get_extra_info

func FunctionGetBindData(info FunctionInfo) unsafe.Pointer {
	ptr := C.duckdb_function_get_bind_data(info.data())
	return unsafe.Pointer(ptr)
}

// duckdb_function_get_init_data

func FunctionGetLocalInitData(info FunctionInfo) unsafe.Pointer {
	ptr := C.duckdb_function_get_local_init_data(info.data())
	return unsafe.Pointer(ptr)
}

func FunctionSetError(info FunctionInfo, err string) {
	cErr := C.CString(err)
	defer Free(unsafe.Pointer(cErr))
	C.duckdb_function_set_error(info.data(), cErr)
}

// ------------------------------------------------------------------ //
// Replacement Scans
// ------------------------------------------------------------------ //

func AddReplacementScan(db Database, callbackPtr unsafe.Pointer, extraData unsafe.Pointer, deleteCallbackPtr unsafe.Pointer) {
	callback := C.duckdb_replacement_callback_t(callbackPtr)
	deleteCallback := C.duckdb_delete_callback_t(deleteCallbackPtr)
	C.duckdb_add_replacement_scan(db.data(), callback, extraData, deleteCallback)
}

func ReplacementScanSetFunctionName(info ReplacementScanInfo, name string) {
	cName := C.CString(name)
	defer Free(unsafe.Pointer(cName))
	C.duckdb_replacement_scan_set_function_name(info.data(), cName)
}

func ReplacementScanAddParameter(info ReplacementScanInfo, v Value) {
	C.duckdb_replacement_scan_add_parameter(info.data(), v.data())
}

func ReplacementScanSetError(info ReplacementScanInfo, err string) {
	cErr := C.CString(err)
	defer Free(unsafe.Pointer(cErr))
	C.duckdb_replacement_scan_set_error(info.data(), cErr)
}

// ------------------------------------------------------------------ //
// Profiling Info
// ------------------------------------------------------------------ //

func GetProfilingInfo(conn Connection) ProfilingInfo {
	info := C.duckdb_get_profiling_info(conn.data())
	return ProfilingInfo{
		Ptr: unsafe.Pointer(info),
	}
}

// duckdb_profiling_info_get_value

// ProfilingInfoGetMetrics wraps duckdb_profiling_info_get_metrics.
// The return value must be destroyed with DestroyValue.
func ProfilingInfoGetMetrics(info ProfilingInfo) Value {
	v := C.duckdb_profiling_info_get_metrics(info.data())
	if debugMode {
		allocCounters.v.Add(1)
	}
	return Value{
		Ptr: unsafe.Pointer(v),
	}
}

func ProfilingInfoGetChildCount(info ProfilingInfo) IdxT {
	return C.duckdb_profiling_info_get_child_count(info.data())
}

func ProfilingInfoGetChild(info ProfilingInfo, index IdxT) ProfilingInfo {
	child := C.duckdb_profiling_info_get_child(info.data(), index)
	return ProfilingInfo{
		Ptr: unsafe.Pointer(child),
	}
}

// ------------------------------------------------------------------ //
// Appender
// ------------------------------------------------------------------ //

// AppenderCreate wraps duckdb_appender_create.
// outAppender must be destroyed with AppenderDestroy.
func AppenderCreate(conn Connection, schema string, table string, outAppender *Appender) State {
	cSchema := C.CString(schema)
	defer Free(unsafe.Pointer(cSchema))
	cTable := C.CString(table)
	defer Free(unsafe.Pointer(cTable))

	var appender C.duckdb_appender
	state := C.duckdb_appender_create(conn.data(), cSchema, cTable, &appender)
	outAppender.Ptr = unsafe.Pointer(appender)
	if debugMode {
		allocCounters.appender.Add(1)
	}
	return state
}

// AppenderCreateExt wraps duckdb_appender_create_ext.
// outAppender must be destroyed with AppenderDestroy.
func AppenderCreateExt(conn Connection, catalog string, schema string, table string, outAppender *Appender) State {
	cCatalog := C.CString(catalog)
	defer Free(unsafe.Pointer(cCatalog))
	cSchema := C.CString(schema)
	defer Free(unsafe.Pointer(cSchema))
	cTable := C.CString(table)
	defer Free(unsafe.Pointer(cTable))

	var appender C.duckdb_appender
	state := C.duckdb_appender_create_ext(conn.data(), cCatalog, cSchema, cTable, &appender)
	outAppender.Ptr = unsafe.Pointer(appender)
	if debugMode {
		allocCounters.appender.Add(1)
	}
	return state
}

func AppenderColumnCount(appender Appender) IdxT {
	return C.duckdb_appender_column_count(appender.data())
}

// AppenderColumnType wraps duckdb_appender_column_type.
// The return value must be destroyed with DestroyLogicalType.
func AppenderColumnType(appender Appender, index IdxT) LogicalType {
	logicalType := C.duckdb_appender_column_type(appender.data(), index)
	if debugMode {
		allocCounters.logicalType.Add(1)
	}
	return LogicalType{
		Ptr: unsafe.Pointer(logicalType),
	}
}

func AppenderError(appender Appender) string {
	err := C.duckdb_appender_error(appender.data())
	return C.GoString(err)
}

func AppenderFlush(appender Appender) State {
	return C.duckdb_appender_flush(appender.data())
}

func AppenderClose(appender Appender) State {
	return C.duckdb_appender_close(appender.data())
}

// AppenderDestroy wraps duckdb_appender_destroy.
func AppenderDestroy(appender *Appender) State {
	if debugMode {
		allocCounters.appender.Add(-1)
	}
	if appender.Ptr == nil {
		return StateSuccess
	}
	data := appender.data()
	state := C.duckdb_appender_destroy(&data)
	appender.Ptr = nil
	return state
}

func AppenderAddColumn(appender Appender, name string) State {
	cName := C.CString(name)
	defer Free(unsafe.Pointer(cName))
	return C.duckdb_appender_add_column(appender.data(), cName)
}

func AppenderClearColumns(appender Appender) State {
	return C.duckdb_appender_clear_columns(appender.data())
}

// duckdb_appender_begin_row
// duckdb_appender_end_row
// duckdb_append_default
// duckdb_append_bool
// duckdb_append_int8
// duckdb_append_int16
// duckdb_append_int32
// duckdb_append_int64
// duckdb_append_hugeint
// duckdb_append_uint8
// duckdb_append_uint16
// duckdb_append_uint32
// duckdb_append_uint64
// duckdb_append_uhugeint
// duckdb_append_float
// duckdb_append_double
// duckdb_append_date
// duckdb_append_time
// duckdb_append_timestamp
// duckdb_append_interval
// duckdb_append_varchar
// duckdb_append_varchar_length
// duckdb_append_blob
// duckdb_append_null
// duckdb_append_value

func AppendDataChunk(appender Appender, chunk DataChunk) State {
	return C.duckdb_append_data_chunk(appender.data(), chunk.data())
}

// ------------------------------------------------------------------ //
// Table Description
// ------------------------------------------------------------------ //

// duckdb_table_description_create

// TableDescriptionCreateExt wraps duckdb_table_description_create_ext.
// outDesc must be destroyed with TableDescriptionDestroy.
func TableDescriptionCreateExt(conn Connection, catalog string, schema string, table string, outDesc *TableDescription) State {
	cCatalog := C.CString(catalog)
	defer Free(unsafe.Pointer(cCatalog))
	cSchema := C.CString(schema)
	defer Free(unsafe.Pointer(cSchema))
	cTable := C.CString(table)
	defer Free(unsafe.Pointer(cTable))

	var description C.duckdb_table_description
	state := C.duckdb_table_description_create_ext(conn.data(), cCatalog, cSchema, cTable, &description)
	outDesc.Ptr = unsafe.Pointer(description)
	if debugMode {
		allocCounters.tableDesc.Add(1)
	}
	return state
}

// TableDescriptionDestroy wraps duckdb_table_description_destroy.
func TableDescriptionDestroy(desc *TableDescription) {
	if debugMode {
		allocCounters.tableDesc.Add(-1)
	}
	if desc.Ptr == nil {
		return
	}
	data := desc.data()
	C.duckdb_table_description_destroy(&data)
	desc.Ptr = nil
}

func TableDescriptionError(desc TableDescription) string {
	err := C.duckdb_table_description_error(desc.data())
	return C.GoString(err)
}

func ColumnHasDefault(desc TableDescription, index IdxT, outBool *bool) State {
	var b C.bool
	state := C.duckdb_column_has_default(desc.data(), index, &b)
	*outBool = bool(b)
	return state
}

func TableDescriptionGetColumnName(desc TableDescription, index IdxT) string {
	cName := C.duckdb_table_description_get_column_name(desc.data(), index)
	defer Free(unsafe.Pointer(cName))
	return C.GoString(cName)
}

//===--------------------------------------------------------------------===//
// Threading Information
//===--------------------------------------------------------------------===//

// duckdb_execute_tasks
// duckdb_create_task_state
// duckdb_execute_tasks_state
// duckdb_execute_n_tasks_state
// duckdb_finish_execution
// duckdb_task_state_is_finished
// duckdb_destroy_task_state
// duckdb_execution_is_finished
// duckdb_fetch_chunk
// duckdb_create_cast_function
// duckdb_cast_function_set_source_type
// duckdb_cast_function_set_target_type
// duckdb_cast_function_set_implicit_cast_cost
// duckdb_cast_function_set_function
// duckdb_cast_function_set_extra_info
// duckdb_cast_function_get_extra_info
// duckdb_cast_function_get_cast_mode
// duckdb_cast_function_set_error
// duckdb_cast_function_set_row_error
// duckdb_register_cast_function
// duckdb_destroy_cast_function

// duckdb_row_count
// duckdb_column_data
// duckdb_nullmask_data

// ResultGetChunk wraps duckdb_result_get_chunk.
// The return value must be destroyed with DestroyDataChunk.
func ResultGetChunk(res Result, index IdxT) DataChunk {
	chunk := C.duckdb_result_get_chunk(res.data, index)
	if debugMode {
		allocCounters.chunk.Add(1)
	}
	return DataChunk{
		Ptr: unsafe.Pointer(chunk),
	}
}

// duckdb_result_is_streaming

func ResultChunkCount(res Result) IdxT {
	return C.duckdb_result_chunk_count(res.data)
}

// duckdb_value_boolean
// duckdb_value_int8
// duckdb_value_int16
// duckdb_value_int32

func ValueInt64(res *Result, col IdxT, row IdxT) int64 {
	v := C.duckdb_value_int64(&res.data, col, row)
	return int64(v)
}

// duckdb_value_hugeint
// duckdb_value_uhugeint
// duckdb_value_decimal
// duckdb_value_uint8
// duckdb_value_uint16
// duckdb_value_uint32
// duckdb_value_uint64
// duckdb_value_float
// duckdb_value_double
// duckdb_value_date
// duckdb_value_time
// duckdb_value_timestamp
// duckdb_value_interval
// duckdb_value_varchar
// duckdb_value_string
// duckdb_value_varchar_internal
// duckdb_value_string_internal
// duckdb_value_blob
// duckdb_value_is_null
// duckdb_execute_prepared_streaming
// duckdb_pending_prepared_streaming

// ------------------------------------------------------------------ //
// Arrow Interface
// ------------------------------------------------------------------ //

// duckdb_query_arrow

func QueryArrowSchema(arrow Arrow, outSchema *ArrowSchema) State {
	return C.duckdb_query_arrow_schema(arrow.data(), (*C.duckdb_arrow_schema)(outSchema.Ptr))
}

// duckdb_prepared_arrow_schema
// duckdb_result_arrow_array

func QueryArrowArray(arrow Arrow, outArray *ArrowArray) State {
	return C.duckdb_query_arrow_array(arrow.data(), (*C.duckdb_arrow_array)(outArray.Ptr))
}

// duckdb_arrow_column_count

func ArrowRowCount(arrow Arrow) IdxT {
	return C.duckdb_arrow_row_count(arrow.data())
}

// duckdb_arrow_rows_changed

func QueryArrowError(arrow Arrow) string {
	err := C.duckdb_query_arrow_error(arrow.data())
	return C.GoString(err)
}

// DestroyArrow wraps duckdb_destroy_arrow.
func DestroyArrow(arrow *Arrow) {
	if debugMode {
		allocCounters.arrow.Add(-1)
	}
	if arrow.Ptr == nil {
		return
	}
	data := arrow.data()
	C.duckdb_destroy_arrow(&data)
	arrow.Ptr = nil
}

// duckdb_destroy_arrow_stream

// ExecutePreparedArrow wraps duckdb_execute_prepared_arrow.
// outArrow must be destroyed with DestroyArrow.
func ExecutePreparedArrow(preparedStmt PreparedStatement, outArrow *Arrow) State {
	var arrow C.duckdb_arrow
	state := C.duckdb_execute_prepared_arrow(preparedStmt.data(), &arrow)
	outArrow.Ptr = unsafe.Pointer(arrow)
	if debugMode {
		allocCounters.arrow.Add(1)
	}
	return state
}

func ArrowScan(conn Connection, table string, stream ArrowStream) State {
	cTable := C.CString(table)
	defer Free(unsafe.Pointer(cTable))
	return C.duckdb_arrow_scan(conn.data(), cTable, stream.data())
}

// duckdb_arrow_array_scan
// duckdb_stream_fetch_chunk

// duckdb_create_instance_cache
// duckdb_get_or_create_from_cache
// duckdb_destroy_instance_cache

// duckdb_append_default_to_chunk

// ------------------------------------------------------------------ //
// Go Bindings Helper
// ------------------------------------------------------------------ //

func ValidityMaskValueIsValid(maskPtr unsafe.Pointer, index IdxT) bool {
	entryIdx := index / 64
	idxInEntry := index % 64
	slice := (*[1 << 31]C.uint64_t)(maskPtr)
	isValid := slice[entryIdx] & (C.uint64_t(1) << idxInEntry)
	return uint64(isValid) != 0
}

const (
	logicalTypeSize = C.size_t(unsafe.Sizeof((C.duckdb_logical_type)(nil)))
	charSize        = C.size_t(unsafe.Sizeof((*C.char)(nil)))
)

func allocLogicalTypeSlice(types []LogicalType) unsafe.Pointer {
	count := len(types)

	// Initialize the memory of the logical types.
	typesSlice := (*[1 << 31]C.duckdb_logical_type)(C.malloc(C.size_t(count) * logicalTypeSize))
	for i, t := range types {
		// We only copy the pointers.
		// The actual types live in types.
		(*typesSlice)[i] = t.data()
	}
	return unsafe.Pointer(typesSlice)
}

// ------------------------------------------------------------------ //
// Memory Safety
// ------------------------------------------------------------------ //

type allocationCounters struct {
	db             atomic.Int64
	conn           atomic.Int64
	config         atomic.Int64
	logicalType    atomic.Int64
	preparedStmt   atomic.Int64
	extractedStmts atomic.Int64
	pendingRes     atomic.Int64
	res            atomic.Int64
	v              atomic.Int64
	chunk          atomic.Int64
	scalarFunc     atomic.Int64
	scalarFuncSet  atomic.Int64
	tableFunc      atomic.Int64
	appender       atomic.Int64
	tableDesc      atomic.Int64
	arrow          atomic.Int64
}

var allocCounters = allocationCounters{}

func VerifyAllocationCounters() {
	dbCount := allocCounters.db.Load()
	if dbCount != 0 {
		log.Panicf("db count is %d", dbCount)
	}
	connCount := allocCounters.conn.Load()
	if connCount != 0 {
		log.Panicf("conn count is %d", connCount)
	}
	configCount := allocCounters.config.Load()
	if configCount != 0 {
		log.Panicf("config count is %d", configCount)
	}
	logicalTypeCount := allocCounters.logicalType.Load()
	if logicalTypeCount != 0 {
		log.Panicf("logical type count is %d", logicalTypeCount)
	}
	preparedStmtCount := allocCounters.preparedStmt.Load()
	if preparedStmtCount != 0 {
		log.Panicf("preparesd statement count is %d", preparedStmtCount)
	}
	extractedStmtsCount := allocCounters.extractedStmts.Load()
	if extractedStmtsCount != 0 {
		log.Panicf("extracted statements count is %d", extractedStmtsCount)
	}
	pendingResCount := allocCounters.pendingRes.Load()
	if pendingResCount != 0 {
		log.Panicf("pending res count is %d", pendingResCount)
	}
	resCount := allocCounters.res.Load()
	if resCount != 0 {
		log.Panicf("res count is %d", resCount)
	}
	vCount := allocCounters.v.Load()
	if vCount != 0 {
		log.Panicf("v count is %d", vCount)
	}
	chunkCount := allocCounters.chunk.Load()
	if chunkCount != 0 {
		log.Panicf("chunk count is %d", chunkCount)
	}
	scalarFuncCount := allocCounters.scalarFunc.Load()
	if scalarFuncCount != 0 {
		log.Panicf("scalar function count is %d", scalarFuncCount)
	}
	scalarFuncSetCount := allocCounters.scalarFuncSet.Load()
	if scalarFuncSetCount != 0 {
		log.Panicf("scalar function set count is %d", scalarFuncSetCount)
	}
	tableFuncCount := allocCounters.tableFunc.Load()
	if tableFuncCount != 0 {
		log.Panicf("table function count is %d", tableFuncCount)
	}
	appenderCount := allocCounters.appender.Load()
	if appenderCount != 0 {
		log.Panicf("appender count is %d", appenderCount)
	}
	tableDescCount := allocCounters.tableDesc.Load()
	if tableDescCount != 0 {
		log.Panicf("table description count is %d", tableDescCount)
	}
	arrowCount := allocCounters.arrow.Load()
	if arrowCount != 0 {
		log.Panicf("arrow count is %d", arrowCount)
	}
}
