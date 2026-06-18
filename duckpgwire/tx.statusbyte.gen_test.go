package duckpgwire

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTxStatusByte(t *testing.T) {
	cases := []struct {
		name  string
		state txState
		want  byte
	}{
		{"idle", txIdle, 'I'},
		{"in_transaction", txInTransaction, 'T'},
		{"failed", txFailed, 'E'},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, c.state.statusByte())
		})
	}
}
