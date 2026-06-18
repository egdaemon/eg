package duckproxy

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestProtocolRoundtrip(t *testing.T) {
	t.Run("client_frame_exec", func(t *testing.T) {
		in := &ClientFrame{Body: &ClientFrame_Exec{Exec: &ExecRequest{Sql: "SELECT 1", Args: []*Param{{Ordinal: 1, Value: &Value{Kind: &Value_IntValue{IntValue: 7}}}}}}}
		var out ClientFrame
		roundtrip(t, in, &out)
		require.Equal(t, "SELECT 1", out.GetExec().GetSql())
		require.EqualValues(t, 7, out.GetExec().GetArgs()[0].GetValue().GetIntValue())
	})

	t.Run("client_frame_query", func(t *testing.T) {
		in := &ClientFrame{Body: &ClientFrame_Query{Query: &QueryRequest{Sql: "SELECT * FROM t"}}}
		var out ClientFrame
		roundtrip(t, in, &out)
		require.Equal(t, "SELECT * FROM t", out.GetQuery().GetSql())
	})

	t.Run("client_frame_begin", func(t *testing.T) {
		in := &ClientFrame{Body: &ClientFrame_Begin{Begin: &BeginRequest{}}}
		var out ClientFrame
		roundtrip(t, in, &out)
		require.NotNil(t, out.GetBegin())
	})

	t.Run("client_frame_commit", func(t *testing.T) {
		in := &ClientFrame{Body: &ClientFrame_Commit{Commit: &CommitRequest{}}}
		var out ClientFrame
		roundtrip(t, in, &out)
		require.NotNil(t, out.GetCommit())
	})

	t.Run("client_frame_rollback", func(t *testing.T) {
		in := &ClientFrame{Body: &ClientFrame_Rollback{Rollback: &RollbackRequest{}}}
		var out ClientFrame
		roundtrip(t, in, &out)
		require.NotNil(t, out.GetRollback())
	})

	t.Run("server_frame_result", func(t *testing.T) {
		in := &ServerFrame{Body: &ServerFrame_Result{Result: &ExecResponse{RowsAffected: 3}}}
		var out ServerFrame
		roundtrip(t, in, &out)
		require.EqualValues(t, 3, out.GetResult().GetRowsAffected())
	})

	t.Run("server_frame_columns", func(t *testing.T) {
		in := &ServerFrame{Body: &ServerFrame_Columns{Columns: &ColumnsResponse{Names: []string{"id", "name"}}}}
		var out ServerFrame
		roundtrip(t, in, &out)
		require.Equal(t, []string{"id", "name"}, out.GetColumns().GetNames())
	})

	t.Run("server_frame_row", func(t *testing.T) {
		in := &ServerFrame{Body: &ServerFrame_Row{Row: &RowResponse{Values: []*Value{{Kind: &Value_StringValue{StringValue: "hi"}}}}}}
		var out ServerFrame
		roundtrip(t, in, &out)
		require.Equal(t, "hi", out.GetRow().GetValues()[0].GetStringValue())
	})

	t.Run("server_frame_done", func(t *testing.T) {
		in := &ServerFrame{Body: &ServerFrame_Done{Done: &DoneResponse{}}}
		var out ServerFrame
		roundtrip(t, in, &out)
		require.NotNil(t, out.GetDone())
	})

	t.Run("server_frame_ok", func(t *testing.T) {
		in := &ServerFrame{Body: &ServerFrame_Ok{Ok: &OkResponse{}}}
		var out ServerFrame
		roundtrip(t, in, &out)
		require.NotNil(t, out.GetOk())
	})

	t.Run("server_frame_error", func(t *testing.T) {
		in := &ServerFrame{Body: &ServerFrame_Error{Error: &ErrorResponse{Message: "boom"}}}
		var out ServerFrame
		roundtrip(t, in, &out)
		require.Equal(t, "boom", out.GetError().GetMessage())
	})

	t.Run("truncated_length_prefix", func(t *testing.T) {
		var buf bytes.Buffer
		buf.Write([]byte{0, 0}) // only 2 of the required 4 length bytes
		var out ServerFrame
		require.Error(t, ReadFrame(&buf, &out))
	})

	t.Run("truncated_body", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, WriteFrame(&buf, &ServerFrame{Body: &ServerFrame_Error{Error: &ErrorResponse{Message: "boom"}}}))

		truncated := bytes.NewReader(buf.Bytes()[:buf.Len()-1])
		var out ServerFrame
		require.Error(t, ReadFrame(truncated, &out))
	})

	t.Run("zero_length_frame", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, WriteFrame(&buf, &ClientFrame{}))

		var out ClientFrame
		require.NoError(t, ReadFrame(&buf, &out))
		require.Nil(t, out.GetBody())

		require.ErrorIs(t, ReadFrame(&buf, &out), io.EOF, "expected io.EOF after consuming the only frame")
	})
}

func roundtrip[T proto.Message](t *testing.T, in T, out T) {
	t.Helper()

	var buf bytes.Buffer
	require.NoError(t, WriteFrame(&buf, in))
	require.NoError(t, ReadFrame(&buf, out))
}
