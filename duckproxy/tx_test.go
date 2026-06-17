package duckproxy

import "testing"

func TestClassifyTxKeyword(t *testing.T) {
	cases := []struct {
		sql  string
		want txKeyword
	}{
		{"BEGIN", txKeywordBegin},
		{"  begin   transaction", txKeywordBegin},
		{"START TRANSACTION", txKeywordBegin},
		{"SELECT 1", txKeywordNone},
		{"COMMIT;", txKeywordCommit},
		{"end", txKeywordCommit},
		{"rollback", txKeywordRollback},
		{"ROLLBACK TO SAVEPOINT x", txKeywordRollback},
		{"  ", txKeywordNone},
		{"INSERT INTO t VALUES (1)", txKeywordNone},
	}

	for _, c := range cases {
		if got := classifyTxKeyword(c.sql); got != c.want {
			t.Errorf("classifyTxKeyword(%q) = %v, want %v", c.sql, got, c.want)
		}
	}
}

func TestTxStateStatusByte(t *testing.T) {
	cases := []struct {
		state txState
		want  byte
	}{
		{txIdle, 'I'},
		{txInTransaction, 'T'},
		{txFailed, 'E'},
	}

	for _, c := range cases {
		if got := c.state.statusByte(); got != c.want {
			t.Errorf("txState(%v).statusByte() = %c, want %c", c.state, got, c.want)
		}
	}
}
