package duckpgwire

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTxClassifyTxKeyword(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want txKeyword
	}{
		{"begin", "BEGIN", txKeywordBegin},
		{"begin_transaction_lowercase", "  begin   transaction", txKeywordBegin},
		{"start_transaction", "START TRANSACTION", txKeywordBegin},
		{"select_is_not_a_keyword", "SELECT 1", txKeywordNone},
		{"commit", "COMMIT;", txKeywordCommit},
		{"end_is_commit", "end", txKeywordCommit},
		{"rollback", "rollback", txKeywordRollback},
		{"rollback_to_savepoint", "ROLLBACK TO SAVEPOINT x", txKeywordRollback},
		{"blank", "  ", txKeywordNone},
		{"insert_is_not_a_keyword", "INSERT INTO t VALUES (1)", txKeywordNone},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, classifyTxKeyword(c.sql))
		})
	}
}
